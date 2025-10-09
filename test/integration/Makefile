# Integration Testing Makefile

.PHONY: help build up down test clean logs shell

# Default target
help: ## Show this help message
	@echo "ExternalSource Controller Integration Tests"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the test environment
	docker-compose build

up: ## Start the integration test environment
	docker-compose up --build

down: ## Stop and remove the test environment
	docker-compose down -v

test: ## Run integration tests (build, test, cleanup)
	docker-compose up --build --abort-on-container-exit
	docker-compose down -v

test-quick: ## Run tests without rebuilding images
	docker-compose up --abort-on-container-exit
	docker-compose down -v

clean: ## Clean up all containers, images, and volumes
	docker-compose down -v --remove-orphans
	docker system prune -f
	docker volume prune -f

logs: ## Show logs from all services
	docker-compose logs -f

logs-k0s: ## Show logs from k0s cluster
	docker-compose logs -f k0s

logs-test: ## Show logs from test runner
	docker-compose logs -f test-runner

logs-api: ## Show logs from test API server
	docker-compose logs -f test-api

shell: ## Open shell in test runner container
	docker exec -it integration-test-runner /bin/bash

shell-k0s: ## Open shell in k0s container
	docker exec -it k0s-cluster /bin/bash

status: ## Show status of all services
	docker-compose ps

# Development targets
dev-up: ## Start environment for development (detached)
	docker-compose up -d k0s test-api minio

dev-test: ## Run tests against running development environment
	docker-compose run --rm test-runner

dev-down: ## Stop development environment
	docker-compose stop k0s test-api minio

# Debugging targets
debug-cluster: ## Get cluster info for debugging
	@echo "=== Cluster Status ==="
	docker exec k0s-cluster k0s kubectl get nodes -o wide || true
	@echo ""
	@echo "=== Pod Status ==="
	docker exec k0s-cluster k0s kubectl get pods --all-namespaces || true
	@echo ""
	@echo "=== Service Status ==="
	docker exec k0s-cluster k0s kubectl get services --all-namespaces || true

debug-flux-external-controller: ## Debug flux-external-controller deployment
	@echo "=== flux-external-controller Deployment ==="
	docker exec k0s-cluster k0s kubectl get deployment -n fx-system || true
	@echo ""
	@echo "=== flux-external-controller Pods ==="
	docker exec k0s-cluster k0s kubectl get pods -n fx-system || true
	@echo ""
	@echo "=== flux-external-controller Logs ==="
	docker exec k0s-cluster k0s kubectl logs -n fx-system deployment/flux-external-controller-manager --tail=50 || true

debug-flux: ## Debug Flux installation
	@echo "=== Flux Pods ==="
	docker exec k0s-cluster k0s kubectl get pods -n flux-system || true
	@echo ""
	@echo "=== Flux Status ==="
	docker exec k0s-cluster flux get all || true

debug-externalsources: ## Show ExternalSource resources
	@echo "=== ExternalSources ==="
	docker exec k0s-cluster k0s kubectl get externalsources -o wide || true
	@echo ""
	@echo "=== ExternalArtifacts ==="
	docker exec k0s-cluster k0s kubectl get externalartifacts -o wide || true

debug-api: ## Test API server endpoints
	@echo "=== API Health Check ==="
	curl -s http://localhost:8080/health || echo "API not accessible"
	@echo ""
	@echo "=== API Config Endpoint ==="
	curl -s http://localhost:8080/api/v1/config | jq . || echo "Config endpoint failed"
	@echo ""
	@echo "=== API Secure Endpoint ==="
	curl -s -H "Authorization: Bearer test-token-123" http://localhost:8080/api/v1/secure-config | jq . || echo "Secure endpoint failed"

# CI/CD targets
ci-test: ## Run tests suitable for CI environment
	@echo "Starting CI integration tests..."
	docker-compose -f docker-compose.yml up --build --abort-on-container-exit --exit-code-from test-runner
	@echo "Cleaning up..."
	docker-compose down -v

ci-build: ## Build images for CI
	docker-compose build --parallel

# Utility targets
wait-for-cluster: ## Wait for k0s cluster to be ready
	@echo "Waiting for k0s cluster..."
	@timeout 300 bash -c 'until docker exec k0s-cluster k0s kubectl get nodes >/dev/null 2>&1; do sleep 5; done'
	@echo "Cluster is ready!"

wait-for-api: ## Wait for test API to be ready
	@echo "Waiting for test API..."
	@timeout 60 bash -c 'until curl -s http://localhost:8080/health >/dev/null 2>&1; do sleep 2; done'
	@echo "API is ready!"

# Example usage targets
example-basic: ## Create a basic ExternalSource example
	docker exec k0s-cluster k0s kubectl apply -f - <<< '\
	apiVersion: source.flux.oddkin.co/v1alpha1\
	kind: ExternalSource\
	metadata:\
	  name: example-basic\
	  namespace: default\
	spec:\
	  interval: 1m\
	  destinationPath: config.json\
	  generator:\
	    type: http\
	    http:\
	      url: http://test-api-service/api/v1/config'

example-auth: ## Create an authenticated ExternalSource example
	docker exec k0s-cluster k0s kubectl create secret generic example-auth --from-literal=Authorization="Bearer test-token-123" --dry-run=client -o yaml | docker exec -i k0s-cluster k0s kubectl apply -f -
	docker exec k0s-cluster k0s kubectl apply -f - <<< '\
	apiVersion: source.flux.oddkin.co/v1alpha1\
	kind: ExternalSource\
	metadata:\
	  name: example-auth\
	  namespace: default\
	spec:\
	  interval: 1m\
	  destinationPath: secure-config.json\
	  generator:\
	    type: http\
	    http:\
	      url: http://test-api-service/api/v1/secure-config\
	      headersSecretRef:\
	        name: example-auth'

example-transform: ## Create a transformation ExternalSource example
	docker exec k0s-cluster k0s kubectl apply -f - <<< '\
	apiVersion: source.flux.oddkin.co/v1alpha1\
	kind: ExternalSource\
	metadata:\
	  name: example-transform\
	  namespace: default\
	spec:\
	  interval: 1m\
	  destinationPath: transformed.yaml\
	  generator:\
	    type: http\
	    http:\
	      url: http://test-api-service/api/v1/settings\
	  transform:\
	    type: cel\
	    expression: |\
	      {\
	        "apiVersion": "v1",\
	        "kind": "ConfigMap",\
	        "metadata": {"name": "app-config"},\
	        "data": {\
	          "app_name": data.app_name,\
	          "version": data.version\
	        }\
	      }'

clean-examples: ## Clean up example resources
	docker exec k0s-cluster k0s kubectl delete externalsource example-basic example-auth example-transform --ignore-not-found=true
	docker exec k0s-cluster k0s kubectl delete secret example-auth --ignore-not-found=true