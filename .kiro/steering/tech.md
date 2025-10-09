# Technology Stack

## Framework & Language
- **Go 1.24.6**: Primary programming language
- **Kubebuilder v4.9.0**: Kubernetes operator framework for scaffolding and code generation
- **Controller Runtime v0.22.1**: Core controller reconciliation framework

## Key Dependencies
- **CEL (Common Expression Language)**: Data transformation engine via `github.com/google/cel-go`
- **Prometheus**: Metrics collection and monitoring via `github.com/prometheus/client_golang`
- **Kubernetes APIs**: Core k8s client libraries (v0.34.0)

## Build System & Tools
- **Make**: Primary build orchestration
- **Docker/Podman**: Container image building (`CONTAINER_TOOL` configurable)
- **Kustomize**: Kubernetes manifest management and deployment
- **golangci-lint**: Code linting and style enforcement

## Testing Framework
- **Ginkgo v2**: BDD testing framework
- **Gomega**: Matcher library for assertions
- **EnvTest**: Kubernetes API server testing environment
- **Kind**: Local Kubernetes clusters for e2e testing

## Common Commands

### Development
```bash
make build          # Build manager binary
make run            # Run controller locally
make test           # Run unit tests
make lint           # Run linter
make lint-fix       # Run linter with auto-fixes
```

### Code Generation
```bash
make generate       # Generate DeepCopy methods
make manifests      # Generate CRDs and RBAC
```

### Docker & Deployment
```bash
make docker-build IMG=<registry/image:tag>  # Build container image
make docker-push IMG=<registry/image:tag>   # Push container image
make install        # Install CRDs to cluster
make deploy IMG=<registry/image:tag>        # Deploy controller to cluster
```

### Testing
```bash
make test-e2e                    # Run e2e tests with Kind
```

## Code Quality
- golangci-lint configuration enforces consistent code style
- Automatic code generation for Kubernetes boilerplate
- Comprehensive test coverage expected for new features