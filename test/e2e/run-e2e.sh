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
    make build
    
    # Build Docker image
    docker build -t "$CONTROLLER_IMAGE" .
    
    # Load image into kind cluster
    log "Loading image into kind cluster..."
    "$KIND_BINARY" load docker-image "$CONTROLLER_IMAGE" --name "$KIND_CLUSTER"
    
    log "Controller image built successfully"
}

# Setup test environment
setup_test_environment() {
    log "Setting up test environment..."
    
    # Create namespace if it doesn't exist
    kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
    
    # Install CRDs
    log "Installing CRDs..."
    make install
    
    # Deploy controller with test image
    log "Deploying controller..."
    make deploy IMG="$CONTROLLER_IMAGE"
    
    # Wait for controller to be ready
    log "Waiting for controller to be ready..."
    kubectl wait --for=condition=available --timeout=300s deployment/controller-manager -n "$NAMESPACE"
    
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
    trap 'cleanup_test_environment; cleanup_kind_cluster; exit $TEST_RESULT' EXIT
    
    check_prerequisites
    setup_kind_cluster
    build_controller_image
    setup_test_environment
    
    # Run tests and capture result
    if run_e2e_tests; then
        log "All E2E tests passed successfully!"
        TEST_RESULT=0
    else
        error "E2E tests failed!"
        TEST_RESULT=1
    fi
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