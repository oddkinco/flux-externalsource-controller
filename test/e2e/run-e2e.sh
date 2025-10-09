#!/bin/bash

# ExternalSource Controller E2E Test Runner
# This script sets up the environment and runs end-to-end tests

set -euo pipefail

# Configuration
CONTROLLER_IMAGE="${CONTROLLER_IMAGE:-externalsource-controller:e2e}"
KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}"
NAMESPACE="${NAMESPACE:-fx-controller-system}"

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
    
    # Check if kubectl is available
    if ! command -v kubectl &> /dev/null; then
        error "kubectl is not installed or not in PATH"
        exit 1
    fi
    
    # Check if we can connect to Kubernetes cluster
    if ! kubectl cluster-info &> /dev/null; then
        error "Cannot connect to Kubernetes cluster. Check your kubeconfig."
        exit 1
    fi
    
    # Check if ginkgo is available
    if ! command -v ginkgo &> /dev/null; then
        warn "ginkgo is not installed. Installing..."
        go install github.com/onsi/ginkgo/v2/ginkgo@latest
    fi
    
    log "Prerequisites check passed"
}

# Build controller image
build_controller_image() {
    log "Building controller image: $CONTROLLER_IMAGE"
    
    # Build the controller binary
    make build
    
    # Build Docker image
    docker build -t "$CONTROLLER_IMAGE" .
    
    # Load image into kind cluster if using kind
    if kubectl config current-context | grep -q "kind"; then
        log "Loading image into kind cluster..."
        kind load docker-image "$CONTROLLER_IMAGE"
    fi
    
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
    
    # Run tests with ginkgo
    cd test/e2e
    ginkgo -v --tags=e2e --timeout=30m --poll-progress-after=60s --poll-progress-interval=10s .
    
    log "E2E tests completed"
}

# Cleanup test environment
cleanup_test_environment() {
    log "Cleaning up test environment..."
    
    # Undeploy controller
    make undeploy || warn "Failed to undeploy controller"
    
    # Uninstall CRDs
    make uninstall || warn "Failed to uninstall CRDs"
    
    # Delete namespace
    kubectl delete namespace "$NAMESPACE" --ignore-not-found=true || warn "Failed to delete namespace"
    
    log "Test environment cleanup complete"
}

# Main execution
main() {
    log "Starting ExternalSource Controller E2E Tests"
    log "Controller Image: $CONTROLLER_IMAGE"
    log "Namespace: $NAMESPACE"
    
    # Trap to ensure cleanup on exit
    trap cleanup_test_environment EXIT
    
    check_prerequisites
    build_controller_image
    setup_test_environment
    run_e2e_tests
    
    log "All E2E tests passed successfully!"
}

# Handle command line arguments
case "${1:-}" in
    "build")
        build_controller_image
        ;;
    "setup")
        setup_test_environment
        ;;
    "test")
        run_e2e_tests
        ;;
    "cleanup")
        cleanup_test_environment
        ;;
    "")
        main
        ;;
    *)
        echo "Usage: $0 [build|setup|test|cleanup]"
        echo "  build   - Build controller image only"
        echo "  setup   - Setup test environment only"
        echo "  test    - Run tests only (assumes environment is ready)"
        echo "  cleanup - Cleanup test environment only"
        echo "  (no args) - Run full e2e test suite"
        exit 1
        ;;
esac