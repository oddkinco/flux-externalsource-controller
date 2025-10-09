#!/bin/bash
set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Wait for k0s cluster to be ready
wait_for_cluster() {
    log_info "Waiting for k0s cluster to be ready..."
    
    # Get kubeconfig from k0s
    for i in {1..30}; do
        if k0s kubectl config view --raw > /root/.kube/config 2>/dev/null; then
            log_success "Retrieved kubeconfig from k0s"
            break
        fi
        log_info "Attempt $i/30: Waiting for k0s to be ready..."
        sleep 10
    done
    
    # Wait for cluster to be responsive
    for i in {1..30}; do
        if kubectl get nodes >/dev/null 2>&1; then
            log_success "Cluster is responsive"
            break
        fi
        log_info "Attempt $i/30: Waiting for cluster API..."
        sleep 10
    done
    
    # Wait for all nodes to be ready
    kubectl wait --for=condition=Ready nodes --all --timeout=300s
    log_success "All nodes are ready"
}

# Install Flux
install_flux() {
    log_info "Installing Flux..."
    
    # Check if Flux is already installed
    if kubectl get namespace flux-system >/dev/null 2>&1; then
        log_info "Flux namespace already exists, checking installation..."
        if kubectl get deployment -n flux-system source-controller >/dev/null 2>&1; then
            log_success "Flux is already installed"
            return 0
        fi
    fi
    
    # Install Flux
    flux install --timeout=5m
    
    # Wait for Flux controllers to be ready
    kubectl wait --for=condition=Available deployment --all -n flux-system --timeout=300s
    log_success "Flux installation completed"
}

# Deploy flux-external-controller
deploy_flux_external_controller() {
    log_info "Deploying flux-external-controller..."
    
    # Create namespace
    kubectl create namespace fx-system --dry-run=client -o yaml | kubectl apply -f -
    
    # Apply CRDs
    kubectl apply -f /test-cases/crds/
    
    # Apply RBAC and deployment
    kubectl apply -f /test-cases/flux-external-controller/
    
    # Wait for deployment to be ready
    kubectl wait --for=condition=Available deployment/flux-external-controller-manager -n fx-system --timeout=300s
    log_success "flux-external-controller deployment completed"
}

# Setup test webserver service
setup_test_services() {
    log_info "Setting up test services..."
    
    # Create service for test API (points to external container)
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: test-api-service
  namespace: default
spec:
  type: ExternalName
  externalName: test-api
  ports:
  - port: 80
    targetPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: minio-service
  namespace: default
spec:
  type: ExternalName
  externalName: minio
  ports:
  - port: 9000
    targetPort: 9000
EOF
    
    log_success "Test services created"
}

# Run test cases
run_test_cases() {
    log_info "Running integration test cases..."
    
    local test_results=()
    local failed_tests=0
    
    # Test 1: Basic HTTP ExternalSource
    log_info "Test 1: Basic HTTP ExternalSource"
    if run_basic_http_test; then
        test_results+=("✓ Basic HTTP test")
    else
        test_results+=("✗ Basic HTTP test")
        ((failed_tests++))
    fi
    
    # Test 2: Authenticated HTTP ExternalSource
    log_info "Test 2: Authenticated HTTP ExternalSource"
    if run_authenticated_http_test; then
        test_results+=("✓ Authenticated HTTP test")
    else
        test_results+=("✗ Authenticated HTTP test")
        ((failed_tests++))
    fi
    
    # Test 3: Data transformation test
    log_info "Test 3: Data transformation test"
    if run_transformation_test; then
        test_results+=("✓ Data transformation test")
    else
        test_results+=("✗ Data transformation test")
        ((failed_tests++))
    fi
    
    # Test 4: Flux integration test
    log_info "Test 4: Flux integration test"
    if run_flux_integration_test; then
        test_results+=("✓ Flux integration test")
    else
        test_results+=("✗ Flux integration test")
        ((failed_tests++))
    fi
    
    # Print results
    log_info "Test Results:"
    for result in "${test_results[@]}"; do
        if [[ $result == ✓* ]]; then
            log_success "$result"
        else
            log_error "$result"
        fi
    done
    
    if [ $failed_tests -eq 0 ]; then
        log_success "All tests passed!"
        return 0
    else
        log_error "$failed_tests test(s) failed"
        return 1
    fi
}

# Test 1: Basic HTTP ExternalSource
run_basic_http_test() {
    log_info "Creating basic HTTP ExternalSource..."
    
    kubectl apply -f - <<EOF
apiVersion: source.flux.oddkin.co/v1alpha1
kind: ExternalSource
metadata:
  name: basic-http-test
  namespace: default
spec:
  interval: 1m
  destinationPath: config.json
  generator:
    type: http
    http:
      url: http://test-api-service/api/v1/config
EOF
    
    # Wait for ExternalSource to be ready
    if wait_for_externalsource_ready "basic-http-test" 120; then
        # Check if ExternalArtifact was created
        if kubectl get externalartifact basic-http-test >/dev/null 2>&1; then
            log_success "ExternalArtifact created successfully"
            return 0
        else
            log_error "ExternalArtifact was not created"
            return 1
        fi
    else
        log_error "ExternalSource did not become ready"
        kubectl describe externalsource basic-http-test
        return 1
    fi
}

# Test 2: Authenticated HTTP ExternalSource
run_authenticated_http_test() {
    log_info "Creating authenticated HTTP ExternalSource..."
    
    # Create secret with auth token
    kubectl create secret generic test-auth-token \
        --from-literal=Authorization="Bearer test-token-123" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    kubectl apply -f - <<EOF
apiVersion: source.flux.oddkin.co/v1alpha1
kind: ExternalSource
metadata:
  name: auth-http-test
  namespace: default
spec:
  interval: 1m
  destinationPath: secure-config.json
  generator:
    type: http
    http:
      url: http://test-api-service/api/v1/secure-config
      headersSecretRef:
        name: test-auth-token
EOF
    
    if wait_for_externalsource_ready "auth-http-test" 120; then
        if kubectl get externalartifact auth-http-test >/dev/null 2>&1; then
            log_success "Authenticated ExternalArtifact created successfully"
            return 0
        else
            log_error "Authenticated ExternalArtifact was not created"
            return 1
        fi
    else
        log_error "Authenticated ExternalSource did not become ready"
        kubectl describe externalsource auth-http-test
        return 1
    fi
}

# Test 3: Data transformation test
run_transformation_test() {
    log_info "Creating ExternalSource with transformation..."
    
    kubectl apply -f - <<EOF
apiVersion: source.flux.oddkin.co/v1alpha1
kind: ExternalSource
metadata:
  name: transform-test
  namespace: default
spec:
  interval: 1m
  destinationPath: transformed-config.yaml
  generator:
    type: http
    http:
      url: http://test-api-service/api/v1/settings
  transform:
    type: cel
    expression: |
      {
        "apiVersion": "v1",
        "kind": "ConfigMap",
        "metadata": {
          "name": "app-settings",
          "namespace": "default"
        },
        "data": {
          "app_name": data.app_name,
          "version": data.version,
          "environment": data.environment,
          "max_connections": string(data.limits.max_connections)
        }
      }
EOF
    
    if wait_for_externalsource_ready "transform-test" 120; then
        if kubectl get externalartifact transform-test >/dev/null 2>&1; then
            log_success "Transformation ExternalArtifact created successfully"
            return 0
        else
            log_error "Transformation ExternalArtifact was not created"
            return 1
        fi
    else
        log_error "Transformation ExternalSource did not become ready"
        kubectl describe externalsource transform-test
        return 1
    fi
}

# Test 4: Flux integration test
run_flux_integration_test() {
    log_info "Testing Flux integration..."
    
    # Create a Kustomization that consumes the ExternalArtifact
    kubectl apply -f - <<EOF
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: external-config-test
  namespace: flux-system
spec:
  interval: 1m
  sourceRef:
    kind: ExternalArtifact
    name: transform-test
    namespace: default
  path: "./"
  prune: true
  targetNamespace: default
EOF
    
    # Wait for Kustomization to be ready
    if wait_for_kustomization_ready "external-config-test" 120; then
        # Check if the ConfigMap was created by Flux
        if kubectl get configmap app-settings >/dev/null 2>&1; then
            log_success "Flux successfully applied ExternalArtifact content"
            return 0
        else
            log_error "ConfigMap was not created by Flux"
            return 1
        fi
    else
        log_error "Kustomization did not become ready"
        kubectl describe kustomization external-config-test -n flux-system
        return 1
    fi
}

# Helper function to wait for ExternalSource to be ready
wait_for_externalsource_ready() {
    local name=$1
    local timeout=${2:-60}
    local count=0
    
    while [ $count -lt $timeout ]; do
        if kubectl get externalsource "$name" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' | grep -q "True"; then
            return 0
        fi
        sleep 5
        ((count += 5))
        log_info "Waiting for ExternalSource $name to be ready... ($count/${timeout}s)"
    done
    
    return 1
}

# Helper function to wait for Kustomization to be ready
wait_for_kustomization_ready() {
    local name=$1
    local timeout=${2:-60}
    local count=0
    
    while [ $count -lt $timeout ]; do
        if kubectl get kustomization "$name" -n flux-system -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' | grep -q "True"; then
            return 0
        fi
        sleep 5
        ((count += 5))
        log_info "Waiting for Kustomization $name to be ready... ($count/${timeout}s)"
    done
    
    return 1
}

# Cleanup function
cleanup() {
    log_info "Cleaning up test resources..."
    
    # Delete test ExternalSources
    kubectl delete externalsource --all --ignore-not-found=true
    kubectl delete externalartifact --all --ignore-not-found=true
    kubectl delete kustomization external-config-test -n flux-system --ignore-not-found=true
    kubectl delete configmap app-settings --ignore-not-found=true
    kubectl delete secret test-auth-token --ignore-not-found=true
    
    log_success "Cleanup completed"
}

# Main execution
main() {
    log_info "Starting ExternalSource Controller integration tests..."
    
    # Setup trap for cleanup
    trap cleanup EXIT
    
    # Run setup steps
    wait_for_cluster
    install_flux
    deploy_flux_external_controller
    setup_test_services
    
    # Run tests
    if run_test_cases; then
        log_success "All integration tests completed successfully!"
        exit 0
    else
        log_error "Some integration tests failed!"
        exit 1
    fi
}

# Run main function
main "$@"