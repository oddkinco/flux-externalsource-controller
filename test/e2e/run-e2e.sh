#!/bin/bash

# ExternalSource Controller E2E Test Runner
# This script sets up the environment and runs end-to-end tests

set -euo pipefail

# Configuration
CONTROLLER_IMAGE="${CONTROLLER_IMAGE:-oddkin.co/flux-externalsource-controller:v0.0.1}"
KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}"
NAMESPACE="${NAMESPACE:-flux-externalsource-controller-system}"
KIND_CLUSTER="${KIND_CLUSTER:-flux-externalsource-controller-e2e}"
KIND_BINARY="${KIND:-kind}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')] $1${NC}"
}

warn() {
    echo -e "${YELLOW}[$(date +'%Y-%m-%d %H:%M:%S')] WARNING: $1${NC}"
}

error() {
    echo -e "${RED}[$(date +'%Y-%m-%d %H:%M:%S')] ERROR: $1${NC}"
}

# Check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."
    
    # Check if Docker is running
    if ! docker info &> /dev/null; then
        error "Docker is not running. Please start Docker Desktop or Docker daemon."
        error "On macOS: Start Docker Desktop application"
        error "On Linux: sudo systemctl start docker"
        exit 1
    fi
    
    # Check if kind is available
    if ! command -v "$KIND_BINARY" &> /dev/null; then
        error "kind is not installed or not in PATH"
        error "Install kind: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
        exit 1
    fi
    
    # Check if kubectl is available
    if ! command -v kubectl &> /dev/null; then
        error "kubectl is not installed or not in PATH"
        error "Install kubectl: https://kubernetes.io/docs/tasks/tools/"
        exit 1
    fi
    
    # Check if ginkgo is available
    if ! command -v ginkgo &> /dev/null; then
        warn "ginkgo is not installed. Installing..."
        go install github.com/onsi/ginkgo/v2/ginkgo@latest
    fi
    
    log "Prerequisites check passed"
}

# Setup kind cluster
setup_kind_cluster() {
    log "Setting up Kind cluster: $KIND_CLUSTER"
    
    # Check if cluster already exists
    if "$KIND_BINARY" get clusters | grep -q "^$KIND_CLUSTER$"; then
        log "Kind cluster '$KIND_CLUSTER' already exists"
    else
        log "Creating Kind cluster '$KIND_CLUSTER'..."
        "$KIND_BINARY" create cluster --name "$KIND_CLUSTER"
    fi
    
    # Set kubeconfig context
    kubectl config use-context "kind-$KIND_CLUSTER"
    
    log "Kind cluster setup complete"
}

# Cleanup kind cluster
cleanup_kind_cluster() {
    log "Cleaning up Kind cluster: $KIND_CLUSTER"
    
    if "$KIND_BINARY" get clusters | grep -q "^$KIND_CLUSTER$"; then
        "$KIND_BINARY" delete cluster --name "$KIND_CLUSTER"
        log "Kind cluster deleted"
    else
        log "Kind cluster '$KIND_CLUSTER' does not exist"
    fi
}

# Build controller image
build_controller_image() {
    log "Building controller image: $CONTROLLER_IMAGE"
    
    # Build the controller binary
    if ! make build; then
        error "Failed to build controller binary"
        return 1
    fi
    
    # Build Docker image
    if ! docker build -t "$CONTROLLER_IMAGE" .; then
        error "Failed to build Docker image"
        return 1
    fi
    
    # Build hook-executor image
    log "Building hook-executor image..."
    if ! docker build -t ghcr.io/oddkinco/hook-executor:latest -f cmd/hook-executor/Dockerfile .; then
        error "Failed to build hook-executor image"
        return 1
    fi
    
    # Load images into kind cluster
    log "Loading images into kind cluster..."
    if ! "$KIND_BINARY" load docker-image "$CONTROLLER_IMAGE" --name "$KIND_CLUSTER"; then
        error "Failed to load controller image into Kind cluster"
        return 1
    fi
    
    if ! "$KIND_BINARY" load docker-image ghcr.io/oddkinco/hook-executor:latest --name "$KIND_CLUSTER"; then
        error "Failed to load hook-executor image into Kind cluster"
        return 1
    fi
    
    log "Controller and hook-executor images built successfully"
}

# Setup test environment
setup_test_environment() {
    log "Setting up test environment..."
    
    # Create namespace if it doesn't exist
    kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
    
    # Install CRDs
    log "Installing CRDs..."
    if ! make install; then
        error "Failed to install CRDs"
        return 1
    fi
    
    # Deploy controller with test image
    log "Deploying controller..."
    if ! make deploy IMG="$CONTROLLER_IMAGE"; then
        error "Failed to deploy controller"
        error "This usually indicates an issue with the Kubernetes manifests"
        return 1
    fi
    
    # Wait for controller to be ready
    log "Waiting for controller to be ready..."
    if ! kubectl wait --for=condition=available --timeout=300s deployment/externalsource-controller-manager -n "$NAMESPACE" 2>/dev/null; then
        error "Controller failed to become ready within timeout"
        warn "Checking deployment status..."
        kubectl get deployment -n "$NAMESPACE" externalsource-controller-manager -o wide || true
        warn "Checking pod status..."
        kubectl get pods -n "$NAMESPACE" -l control-plane=controller-manager -o wide || true
        warn "Checking pod description..."
        kubectl describe pods -n "$NAMESPACE" -l control-plane=controller-manager || true
        warn "Checking controller logs..."
        kubectl logs -n "$NAMESPACE" deployment/externalsource-controller-manager --tail=50 --all-containers || true
        return 1
    fi
    
    log "Test environment setup complete"
}

# Run e2e tests
run_e2e_tests() {
    log "Running E2E tests..."
    
    # Set environment variables for tests
    export PROJECT_IMAGE="$CONTROLLER_IMAGE"
    export KIND_CLUSTER="$KIND_CLUSTER"
    
    # Run tests with ginkgo
    cd test/e2e
    ginkgo -v --tags=e2e --timeout=30m --poll-progress-after=60s --poll-progress-interval=10s .
    
    log "E2E tests completed"
}

# Cleanup test environment
cleanup_test_environment() {
    log "Cleaning up test environment..."
    
    # Undeploy controller
    make undeploy ignore-not-found=true || warn "Failed to undeploy controller"
    
    # Uninstall CRDs
    make uninstall ignore-not-found=true || warn "Failed to uninstall CRDs"
    
    # Delete namespace
    kubectl delete namespace "$NAMESPACE" --ignore-not-found=true || warn "Failed to delete namespace"
    
    log "Test environment cleanup complete"
}

# Main execution
main() {
    log "Starting ExternalSource Controller E2E Tests"
    log "Controller Image: $CONTROLLER_IMAGE"
    log "Kind Cluster: $KIND_CLUSTER"
    log "Namespace: $NAMESPACE"
    
    # Track test result separately from cleanup
    TEST_RESULT=0
    
    # Trap to ensure cleanup on exit
    trap 'cleanup_on_exit' EXIT
    
    check_prerequisites
    
    if ! setup_kind_cluster; then
        error "=========================================="
        error "CLUSTER SETUP FAILED"
        error "=========================================="
        TEST_RESULT=1
        return
    fi
    
    if ! build_controller_image; then
        error "=========================================="
        error "IMAGE BUILD FAILED"
        error "=========================================="
        TEST_RESULT=1
        return
    fi
    
    # Setup environment with error handling
    if ! setup_test_environment; then
        error "=========================================="
        error "SETUP FAILED - E2E tests cannot continue"
        error "=========================================="
        TEST_RESULT=1
        return
    fi
    
    # Run tests and capture result
    if run_e2e_tests; then
        log "=========================================="
        log "All E2E tests passed successfully!"
        log "=========================================="
        TEST_RESULT=0
    else
        error "=========================================="
        error "E2E TESTS FAILED"
        error "=========================================="
        TEST_RESULT=1
    fi
}

# Cleanup on exit handler
cleanup_on_exit() {
    EXIT_CODE=$?
    
    if [ $TEST_RESULT -ne 0 ] || [ $EXIT_CODE -ne 0 ]; then
        error "=========================================="
        error "E2E TEST RUN FAILED (exit code: $EXIT_CODE)"
        error "=========================================="
    fi
    
    cleanup_test_environment
    cleanup_kind_cluster
    
    exit $TEST_RESULT
}

# Handle command line arguments
case "${1:-}" in
    "build")
        check_prerequisites
        setup_kind_cluster
        build_controller_image
        ;;
    "setup")
        check_prerequisites
        setup_kind_cluster
        setup_test_environment
        ;;
    "test")
        run_e2e_tests
        ;;
    "cleanup")
        cleanup_test_environment
        cleanup_kind_cluster
        ;;
    "")
        main
        ;;
    *)
        echo "Usage: $0 [build|setup|test|cleanup]"
        echo "  build   - Build controller image and setup kind cluster"
        echo "  setup   - Setup kind cluster and test environment"
        echo "  test    - Run tests only (assumes environment is ready)"
        echo "  cleanup - Cleanup test environment and kind cluster"
        echo "  (no args) - Run full e2e test suite"
        exit 1
        ;;
esac