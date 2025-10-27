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
    
    # Get the project root directory (two levels up from test/e2e if called from there)
    if [[ "$PWD" == */test/e2e ]]; then
        PROJECT_ROOT="$(cd ../.. && pwd)"
    else
        PROJECT_ROOT="$(pwd)"
    fi
    
    # Build the controller binary
    log "Building from directory: $PROJECT_ROOT"
    if ! (cd "$PROJECT_ROOT" && make build); then
        error "Failed to build controller binary"
        return 1
    fi
    
    # Build Docker image
    if ! (cd "$PROJECT_ROOT" && docker build -t "$CONTROLLER_IMAGE" .); then
        error "Failed to build Docker image"
        return 1
    fi
    
    # Build externalsource-hook-executor image
    log "Building externalsource-hook-executor image..."
    if ! (cd "$PROJECT_ROOT" && docker build -t ghcr.io/oddkinco/externalsource-hook-executor:latest -f cmd/externalsource-hook-executor/Dockerfile .); then
        error "Failed to build externalsource-hook-executor image"
        return 1
    fi
    
    # Load images into kind cluster
    log "Loading images into kind cluster..."
    if ! "$KIND_BINARY" load docker-image "$CONTROLLER_IMAGE" --name "$KIND_CLUSTER"; then
        error "Failed to load controller image into Kind cluster"
        return 1
    fi
    
    if ! "$KIND_BINARY" load docker-image ghcr.io/oddkinco/externalsource-hook-executor:latest --name "$KIND_CLUSTER"; then
        error "Failed to load externalsource-hook-executor image into Kind cluster"
        return 1
    fi
    
    log "Controller and externalsource-hook-executor images built successfully"
}

# Setup test environment
setup_test_environment() {
    log "Test environment ready - images loaded into Kind cluster"
    log "Ginkgo tests will handle namespace creation, CRD installation, and controller deployment"
}

# Run e2e tests
run_e2e_tests() {
    log "Running E2E tests..."
    
    # Get the project root directory
    if [[ "$PWD" == */test/e2e ]]; then
        PROJECT_ROOT="$(cd ../.. && pwd)"
    else
        PROJECT_ROOT="$(pwd)"
    fi
    
    # Set environment variables for tests
    export PROJECT_IMAGE="$CONTROLLER_IMAGE"
    export KIND_CLUSTER="$KIND_CLUSTER"
    
    # Ensure kubectl uses the Kind cluster context
    kubectl config use-context "kind-$KIND_CLUSTER" || {
        error "Failed to set kubectl context to kind-$KIND_CLUSTER"
        return 1
    }
    
    # Verify cluster connectivity
    log "Verifying cluster connectivity..."
    if ! kubectl cluster-info &> /dev/null; then
        error "Cannot connect to Kind cluster. Is it running?"
        kubectl config current-context || error "No kubectl context set"
        return 1
    fi
    
    log "Cluster is accessible. Current context: $(kubectl config current-context)"
    
    # Run tests with ginkgo from project root
    cd "$PROJECT_ROOT/test/e2e"
    log "Running ginkgo tests from: $PWD"
    ginkgo -v --tags=e2e --timeout=30m --poll-progress-after=60s --poll-progress-interval=10s .
    TEST_EXIT_CODE=$?
    
    # Return to project root
    cd "$PROJECT_ROOT"
    
    if [ $TEST_EXIT_CODE -eq 0 ]; then
        log "E2E tests completed successfully"
    else
        error "E2E tests failed with exit code: $TEST_EXIT_CODE"
    fi
    
    return $TEST_EXIT_CODE
}

# Cleanup test environment
cleanup_test_environment() {
    log "Cleaning up test environment..."
    
    # Get the project root directory (two levels up from test/e2e)
    PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
    
    # Set kubectl context if cluster exists
    if "$KIND_BINARY" get clusters 2>/dev/null | grep -q "^$KIND_CLUSTER$"; then
        kubectl config use-context "kind-$KIND_CLUSTER" || warn "Failed to set kubectl context"
    fi
    
    # Undeploy controller
    (cd "$PROJECT_ROOT" && make undeploy ignore-not-found=true) || warn "Failed to undeploy controller"
    
    # Uninstall CRDs
    (cd "$PROJECT_ROOT" && make uninstall ignore-not-found=true) || warn "Failed to uninstall CRDs"
    
    # Delete namespace
    kubectl delete namespace "$NAMESPACE" --ignore-not-found=true --timeout=30s || warn "Failed to delete namespace"
    
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