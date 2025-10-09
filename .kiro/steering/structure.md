# Project Structure

## Root Level
- `cmd/main.go`: Application entry point
- `Makefile`: Build system and common tasks
- `Dockerfile`: Container image definition
- `go.mod/go.sum`: Go module dependencies
- `PROJECT`: Kubebuilder project metadata

## API Definitions (`api/v1alpha1/`)
- `externalsource_types.go`: ExternalSource CRD definition
- `externalartifact_types.go`: ExternalArtifact CRD definition
- `groupversion_info.go`: API group metadata
- `zz_generated.deepcopy.go`: Auto-generated DeepCopy methods

## Internal Architecture (`internal/`)

### Modular Components
- `artifact/`: Artifact packaging and management interfaces
- `config/`: Configuration management and ConfigMap handling
- `controller/`: Kubernetes controller reconciliation logic
- `generator/`: Pluggable source generators (HTTP, future extensions)
- `metrics/`: Observability and Prometheus metrics
- `storage/`: Storage backend abstractions (memory, S3)
- `transformer/`: Data transformation (CEL-based)

### Architecture Patterns
- **Interface-driven design**: Each component defines clear interfaces for extensibility
- **Factory pattern**: Used in generators for pluggable source types
- **Strategy pattern**: Applied to storage backends and transformers
- **Dependency injection**: Components accept interfaces, not concrete types

## Configuration (`config/`)
- `crd/`: Custom Resource Definitions
- `rbac/`: Role-based access control manifests
- `manager/`: Controller deployment configuration
- `samples/`: Example resource instances
- `default/`: Default Kustomize overlay
- `development/production/`: Environment-specific overlays

## Testing (`test/`)
- `e2e/`: End-to-end integration tests
- `utils/`: Shared testing utilities
- Unit tests co-located with source files (`*_test.go`)

## Code Organization Rules
- Follow standard Go project layout
- Keep interfaces small and focused
- Use dependency injection for testability
- Separate concerns into distinct packages
- Co-locate tests with implementation files
- Use Kubebuilder markers for code generation