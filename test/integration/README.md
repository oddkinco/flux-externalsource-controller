# Integration Testing Environment

This directory contains a complete Docker Compose-based integration testing environment for the ExternalSource Controller.

## Overview

The integration test environment includes:

- **k0s Kubernetes cluster**: Lightweight Kubernetes distribution running in a container
- **Flux installation**: GitOps toolkit for consuming ExternalArtifact resources
- **Test API server**: HTTP server providing various endpoints for testing
- **MinIO**: S3-compatible storage for artifact storage testing
- **flux-externalsource-controller**: The ExternalSource Controller being tested

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   k0s cluster   │    │   Test API      │    │     MinIO       │
│                 │    │   Server        │    │   (S3 storage)  │
│ - Kubernetes    │    │                 │    │                 │
│ - Flux          │    │ - /api/v1/config│    │ - Artifact      │
│ - flux-external-controller │    │ - /api/v1/users │    │   storage       │
│                 │    │ - /health       │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────┐
                    │  Test Runner    │
                    │                 │
                    │ - Run test cases│
                    │ - Verify results│
                    │ - Generate report│
                    └─────────────────┘
```

## Quick Start

### Prerequisites

- Docker and Docker Compose
- At least 4GB of available RAM
- Ports 6443, 8080, 9000, 9001 available on the host

### Running the Tests

1. **Start the environment:**
   ```bash
   cd test/integration
   docker-compose up --build
   ```

2. **View test results:**
   The test runner will automatically execute all integration tests and display results.

3. **Clean up:**
   ```bash
   docker-compose down -v
   ```

## Test Cases

### Test 1: Basic HTTP ExternalSource

Tests the basic functionality of fetching data from an HTTP endpoint:

- Creates an ExternalSource pointing to `/api/v1/config`
- Verifies that an ExternalArtifact is created
- Checks that the artifact contains the expected data

### Test 2: Authenticated HTTP ExternalSource

Tests HTTP authentication using Kubernetes secrets:

- Creates a secret with authentication headers
- Creates an ExternalSource with `headersSecretRef`
- Verifies successful authentication and artifact creation

### Test 3: Data Transformation

Tests CEL-based data transformation:

- Fetches data from `/api/v1/settings`
- Applies CEL transformation to convert to ConfigMap format
- Verifies the transformed output structure

### Test 4: Flux Integration

Tests end-to-end GitOps workflow:

- Creates a Flux Kustomization consuming an ExternalArtifact
- Verifies that Flux applies the artifact content to the cluster
- Checks that the expected Kubernetes resources are created

## Manual Testing

You can also run tests manually by connecting to the running environment:

### Access the k0s cluster

```bash
# Get the kubeconfig
docker exec k0s-cluster k0s kubectl config view --raw > kubeconfig

# Use kubectl with the cluster
export KUBECONFIG=./kubeconfig
kubectl get nodes
```

### Test the API server

```bash
# Test basic endpoints
curl http://localhost:8080/api/v1/config
curl http://localhost:8080/api/v1/users

# Test authenticated endpoint
curl -H "Authorization: Bearer test-token-123" \
     http://localhost:8080/api/v1/secure-config
```

### Access MinIO

- Web UI: http://localhost:9001
- Username: `minioadmin`
- Password: `minioadmin123`

## Test Configuration

### Test API Endpoints

The test API server provides several endpoints for different test scenarios:

- `GET /api/v1/config` - Basic configuration data
- `GET /api/v1/users` - User data array
- `GET /api/v1/settings` - Application settings
- `GET /api/v1/secure-config` - Requires authentication
- `GET /api/v1/environment-config?env=<env>` - Environment-specific config
- `GET /api/v1/slow-config` - Simulates slow responses (2s delay)
- `GET /api/v1/flaky-config` - Randomly returns errors (30% failure rate)

### Environment Variables

The test environment supports several configuration options:

- `STORAGE_BACKEND`: Set to `memory` or `s3` (default: `memory`)
- `HTTP_TIMEOUT`: HTTP request timeout (default: `30s`)
- `TRANSFORM_TIMEOUT`: Transformation timeout (default: `10s`)

## Troubleshooting

### Common Issues

1. **Cluster startup timeout:**
   - Increase memory allocation to Docker
   - Check that required ports are not in use
   - Wait longer for k0s to initialize

2. **Test failures:**
   - Check controller logs: `docker logs k0s-cluster`
   - Verify API server is responding: `curl http://localhost:8080/health`
   - Check Flux installation: `kubectl get pods -n flux-system`

3. **Resource constraints:**
   - Ensure at least 4GB RAM is available
   - Close other Docker containers to free resources

### Debugging

1. **Access the test runner:**
   ```bash
   docker exec -it integration-test-runner /bin/bash
   ```

2. **Check controller logs:**
   ```bash
   kubectl logs -n fx-system deployment/flux-external-controller-manager -f
   ```

3. **Inspect ExternalSource status:**
   ```bash
   kubectl describe externalsource <name>
   ```

4. **View Flux status:**
   ```bash
   flux get all
   ```

## Extending the Tests

### Adding New Test Cases

1. Add test functions to `scripts/run-integration-tests.sh`
2. Create corresponding ExternalSource manifests
3. Add verification logic for expected outcomes

### Adding New API Endpoints

1. Update `nginx.conf` with new location blocks
2. Add corresponding test data files
3. Update the test cases to use new endpoints

### Testing Different Storage Backends

1. Update `docker-compose.yml` to configure S3 storage
2. Set `STORAGE_BACKEND=s3` environment variable
3. Add MinIO bucket creation to setup scripts

## CI/CD Integration

This integration test environment can be used in CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
name: Integration Tests
on: [push, pull_request]

jobs:
  integration-test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    - name: Run integration tests
      run: |
        cd test/integration
        docker-compose up --build --abort-on-container-exit
        docker-compose down -v
```

The test environment is designed to be deterministic and suitable for automated testing in CI/CD environments.