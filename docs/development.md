# Developer Documentation

This document provides comprehensive guidance for developers who want to extend the ExternalSource Controller or contribute to its development.

## Architecture Overview

The ExternalSource Controller follows a modular architecture that enables easy extension through well-defined interfaces. The key components are:

- **Source Generators**: Pluggable implementations for different external data sources
- **Transformers**: Data transformation engines (currently CEL-based)
- **Storage Backends**: Artifact storage implementations (S3, memory)
- **Artifact Manager**: Packaging and lifecycle management
- **Reconciler**: Core Kubernetes controller logic

## Development Environment Setup

### Prerequisites

- Go 1.24+
- Docker or Podman
- kubectl
- Kind (for local testing)
- Make

### Local Development

1. **Clone and setup:**
   ```bash
   git clone https://github.com/example/fx-controller.git
   cd fx-controller
   go mod download
   ```

2. **Install development tools:**
   ```bash
   make install-tools
   ```

3. **Run tests:**
   ```bash
   make test
   make test-integration
   ```

4. **Run locally against a cluster:**
   ```bash
   make install  # Install CRDs
   make run      # Run controller locally
   ```

### Code Generation

The project uses Kubebuilder for code generation:

```bash
# Generate DeepCopy methods
make generate

# Generate CRDs and RBAC
make manifests

# Update API documentation
make api-docs
```

## Adding New Source Generator Types

The most common extension is adding new source generator types. Here's a complete guide:

### 1. Define the Generator Interface

All source generators must implement the `SourceGenerator` interface:

```go
// internal/generator/interface.go
type SourceGenerator interface {
    // Generate fetches data from the external source
    Generate(ctx context.Context, config GeneratorConfig) (*SourceData, error)
    
    // SupportsConditionalFetch indicates if the generator supports ETag-like optimization
    SupportsConditionalFetch() bool
    
    // GetLastModified returns the last modification identifier for conditional fetching
    GetLastModified(ctx context.Context, config GeneratorConfig) (string, error)
}

type GeneratorConfig struct {
    Type   string                 `json:"type"`
    Config map[string]interface{} `json:"config"`
}

type SourceData struct {
    Data         []byte            `json:"data"`
    LastModified string            `json:"lastModified,omitempty"`
    Metadata     map[string]string `json:"metadata,omitempty"`
}
```

### 2. Implement Your Generator

Create a new file `internal/generator/yourtype.go`:

```go
package generator

import (
    "context"
    "fmt"
    "encoding/json"
)

// S3Generator implements SourceGenerator for S3-compatible storage
type S3Generator struct {
    client S3Client
}

// S3Config defines the configuration for S3 sources
type S3Config struct {
    Bucket    string `json:"bucket"`
    Key       string `json:"key"`
    Region    string `json:"region,omitempty"`
    Endpoint  string `json:"endpoint,omitempty"`
}

func NewS3Generator(client S3Client) *S3Generator {
    return &S3Generator{client: client}
}

func (g *S3Generator) Generate(ctx context.Context, config GeneratorConfig) (*SourceData, error) {
    // Parse S3-specific configuration
    var s3Config S3Config
    configBytes, err := json.Marshal(config.Config)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal config: %w", err)
    }
    
    if err := json.Unmarshal(configBytes, &s3Config); err != nil {
        return nil, fmt.Errorf("failed to parse S3 config: %w", err)
    }
    
    // Validate required fields
    if s3Config.Bucket == "" || s3Config.Key == "" {
        return nil, fmt.Errorf("bucket and key are required for S3 generator")
    }
    
    // Fetch data from S3
    data, metadata, err := g.client.GetObject(ctx, s3Config.Bucket, s3Config.Key)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch S3 object: %w", err)
    }
    
    return &SourceData{
        Data:         data,
        LastModified: metadata.ETag,
        Metadata: map[string]string{
            "bucket":      s3Config.Bucket,
            "key":         s3Config.Key,
            "contentType": metadata.ContentType,
        },
    }, nil
}

func (g *S3Generator) SupportsConditionalFetch() bool {
    return true // S3 supports ETags
}

func (g *S3Generator) GetLastModified(ctx context.Context, config GeneratorConfig) (string, error) {
    var s3Config S3Config
    configBytes, err := json.Marshal(config.Config)
    if err != nil {
        return "", err
    }
    
    if err := json.Unmarshal(configBytes, &s3Config); err != nil {
        return "", err
    }
    
    metadata, err := g.client.HeadObject(ctx, s3Config.Bucket, s3Config.Key)
    if err != nil {
        return "", err
    }
    
    return metadata.ETag, nil
}

// S3Client interface for dependency injection and testing
type S3Client interface {
    GetObject(ctx context.Context, bucket, key string) ([]byte, *ObjectMetadata, error)
    HeadObject(ctx context.Context, bucket, key string) (*ObjectMetadata, error)
}

type ObjectMetadata struct {
    ETag        string
    ContentType string
    Size        int64
}
```

### 3. Add Configuration Validation

Update the CRD schema to include your new generator type:

```go
// api/v1alpha1/externalsource_types.go

// Add to the Generator struct
type Generator struct {
    Type string `json:"type"`
    
    // +optional
    HTTP *HTTPGenerator `json:"http,omitempty"`
    
    // +optional
    S3 *S3Generator `json:"s3,omitempty"`  // Add your new generator
}

// Define the configuration struct
type S3Generator struct {
    // +kubebuilder:validation:Required
    Bucket string `json:"bucket"`
    
    // +kubebuilder:validation:Required
    Key string `json:"key"`
    
    // +optional
    Region string `json:"region,omitempty"`
    
    // +optional
    Endpoint string `json:"endpoint,omitempty"`
}
```

Update the CRD validation:

```go
// +kubebuilder:validation:Enum=http;s3
Type string `json:"type"`
```

### 4. Register the Generator

Register your generator in the factory:

```go
// internal/controller/externalsource_controller.go

func (r *ExternalSourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
    // Initialize the factory
    r.factory = generator.NewFactory()
    
    // Register built-in generators
    r.factory.RegisterGenerator("http", func() generator.SourceGenerator {
        return generator.NewHTTPGenerator(r.httpClient)
    })
    
    // Register your new generator
    r.factory.RegisterGenerator("s3", func() generator.SourceGenerator {
        return generator.NewS3Generator(r.s3Client)
    })
    
    return ctrl.NewControllerManagedBy(mgr).
        For(&sourcev1alpha1.ExternalSource{}).
        Complete(r)
}
```

### 5. Add Tests

Create comprehensive tests for your generator:

```go
// internal/generator/s3_test.go
package generator

import (
    "context"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

// Mock S3 client for testing
type MockS3Client struct {
    mock.Mock
}

func (m *MockS3Client) GetObject(ctx context.Context, bucket, key string) ([]byte, *ObjectMetadata, error) {
    args := m.Called(ctx, bucket, key)
    return args.Get(0).([]byte), args.Get(1).(*ObjectMetadata), args.Error(2)
}

func (m *MockS3Client) HeadObject(ctx context.Context, bucket, key string) (*ObjectMetadata, error) {
    args := m.Called(ctx, bucket, key)
    return args.Get(0).(*ObjectMetadata), args.Error(1)
}

func TestS3Generator_Generate(t *testing.T) {
    tests := []struct {
        name           string
        config         GeneratorConfig
        mockSetup      func(*MockS3Client)
        expectedData   []byte
        expectedError  string
    }{
        {
            name: "successful fetch",
            config: GeneratorConfig{
                Type: "s3",
                Config: map[string]interface{}{
                    "bucket": "test-bucket",
                    "key":    "config.json",
                },
            },
            mockSetup: func(m *MockS3Client) {
                m.On("GetObject", mock.Anything, "test-bucket", "config.json").
                    Return([]byte(`{"test": "data"}`), &ObjectMetadata{
                        ETag:        "abc123",
                        ContentType: "application/json",
                    }, nil)
            },
            expectedData: []byte(`{"test": "data"}`),
        },
        {
            name: "missing bucket",
            config: GeneratorConfig{
                Type: "s3",
                Config: map[string]interface{}{
                    "key": "config.json",
                },
            },
            expectedError: "bucket and key are required",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockClient := &MockS3Client{}
            if tt.mockSetup != nil {
                tt.mockSetup(mockClient)
            }
            
            generator := NewS3Generator(mockClient)
            
            result, err := generator.Generate(context.Background(), tt.config)
            
            if tt.expectedError != "" {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.expectedError)
                return
            }
            
            assert.NoError(t, err)
            assert.Equal(t, tt.expectedData, result.Data)
            mockClient.AssertExpectations(t)
        })
    }
}
```

### 6. Update Documentation

Add your generator to the examples and documentation:

```yaml
# examples/s3-source.yaml
apiVersion: source.example.com/v1alpha1
kind: ExternalSource
metadata:
  name: s3-config-source
spec:
  interval: 10m
  generator:
    type: s3
    s3:
      bucket: my-config-bucket
      key: app/config.json
      region: us-west-2
```

## Adding New Transformer Types

To add a new transformation engine (e.g., JSONPath, Lua):

### 1. Implement the Transformer Interface

```go
// internal/transformer/interface.go
type Transformer interface {
    Transform(ctx context.Context, input []byte, expression string) ([]byte, error)
}
```

### 2. Create Your Implementation

```go
// internal/transformer/jsonpath.go
package transformer

import (
    "context"
    "encoding/json"
    "fmt"
    
    "k8s.io/client-go/util/jsonpath"
)

type JSONPathTransformer struct{}

func NewJSONPathTransformer() *JSONPathTransformer {
    return &JSONPathTransformer{}
}

func (t *JSONPathTransformer) Transform(ctx context.Context, input []byte, expression string) ([]byte, error) {
    // Parse input JSON
    var data interface{}
    if err := json.Unmarshal(input, &data); err != nil {
        return nil, fmt.Errorf("failed to parse input JSON: %w", err)
    }
    
    // Create JSONPath parser
    jp := jsonpath.New("transform")
    if err := jp.Parse(expression); err != nil {
        return nil, fmt.Errorf("failed to parse JSONPath expression: %w", err)
    }
    
    // Execute transformation
    results, err := jp.FindResults(data)
    if err != nil {
        return nil, fmt.Errorf("failed to execute JSONPath: %w", err)
    }
    
    // Extract first result
    if len(results) == 0 || len(results[0]) == 0 {
        return nil, fmt.Errorf("JSONPath expression returned no results")
    }
    
    result := results[0][0].Interface()
    
    // Marshal result back to JSON
    output, err := json.Marshal(result)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal result: %w", err)
    }
    
    return output, nil
}
```

## Adding New Storage Backends

To add a new storage backend (e.g., Azure Blob, GCS):

### 1. Implement the Storage Interface

```go
// internal/storage/interface.go
type StorageBackend interface {
    Store(ctx context.Context, key string, data []byte) (string, error)
    List(ctx context.Context, prefix string) ([]string, error)
    Delete(ctx context.Context, key string) error
    GetURL(key string) string
}
```

### 2. Create Your Implementation

```go
// internal/storage/azure.go
package storage

import (
    "context"
    "fmt"
    "strings"
    
    "github.com/Azure/azure-storage-blob-go/azblob"
)

type AzureBlobBackend struct {
    containerURL azblob.ContainerURL
    accountName  string
    containerName string
}

func NewAzureBlobBackend(accountName, containerName, accountKey string) (*AzureBlobBackend, error) {
    credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
    if err != nil {
        return nil, err
    }
    
    pipeline := azblob.NewPipeline(credential, azblob.PipelineOptions{})
    
    containerURL := azblob.NewContainerURL(
        fmt.Sprintf("https://%s.blob.core.windows.net/%s", accountName, containerName),
        pipeline,
    )
    
    return &AzureBlobBackend{
        containerURL:  containerURL,
        accountName:   accountName,
        containerName: containerName,
    }, nil
}

func (b *AzureBlobBackend) Store(ctx context.Context, key string, data []byte) (string, error) {
    blobURL := b.containerURL.NewBlobURL(key)
    
    _, err := azblob.UploadBufferToBlockBlob(ctx, data, blobURL, azblob.UploadToBlockBlobOptions{})
    if err != nil {
        return "", fmt.Errorf("failed to upload blob: %w", err)
    }
    
    return b.GetURL(key), nil
}

func (b *AzureBlobBackend) GetURL(key string) string {
    return fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s", 
        b.accountName, b.containerName, key)
}

// Implement other interface methods...
```

## Testing Guidelines

### Unit Tests

- Test all public methods with various inputs
- Use dependency injection and mocking for external dependencies
- Aim for >80% code coverage
- Include edge cases and error scenarios

### Integration Tests

- Test complete workflows with real external dependencies
- Use testcontainers for external services (MinIO, HTTP servers)
- Validate end-to-end functionality

### Example Test Structure

```go
func TestExternalSourceController_Reconcile(t *testing.T) {
    tests := []struct {
        name           string
        externalSource *sourcev1alpha1.ExternalSource
        mockSetup      func(*MockGenerator, *MockStorage)
        expectedStatus sourcev1alpha1.ExternalSourceStatus
        expectedError  string
    }{
        // Test cases...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup mocks and test environment
            // Run reconciliation
            // Assert results
        })
    }
}
```

## Code Style and Conventions

### Go Code Style

- Follow standard Go conventions (gofmt, golint)
- Use meaningful variable and function names
- Add comprehensive comments for public APIs
- Handle errors explicitly and provide context

### Kubernetes Conventions

- Use Kubebuilder markers for code generation
- Follow Kubernetes API conventions for CRDs
- Implement proper status conditions
- Use structured logging with consistent fields

### Example Code Style

```go
// HTTPGenerator implements SourceGenerator for HTTP-based external sources.
// It supports various authentication methods, TLS configuration, and conditional
// fetching using ETags for optimization.
type HTTPGenerator struct {
    client HTTPClient
    logger logr.Logger
}

// Generate fetches data from an HTTP endpoint according to the provided configuration.
// It returns the response data along with metadata for caching and versioning.
func (g *HTTPGenerator) Generate(ctx context.Context, config GeneratorConfig) (*SourceData, error) {
    log := g.logger.WithValues("generator", "http", "url", config.URL)
    log.V(1).Info("Starting HTTP request")
    
    // Implementation with proper error handling and logging
    
    log.V(1).Info("HTTP request completed", "size", len(data))
    return result, nil
}
```

## Contributing Guidelines

### Pull Request Process

1. **Fork and branch**: Create a feature branch from main
2. **Implement**: Add your feature with tests and documentation
3. **Test**: Ensure all tests pass and add new tests for your changes
4. **Document**: Update relevant documentation and examples
5. **Submit**: Create a pull request with a clear description

### Code Review Checklist

- [ ] Code follows project conventions and style
- [ ] All tests pass and new tests are added
- [ ] Documentation is updated
- [ ] Breaking changes are clearly marked
- [ ] Performance impact is considered
- [ ] Security implications are reviewed

### Release Process

1. **Version bump**: Update version in relevant files
2. **Changelog**: Update CHANGELOG.md with new features and fixes
3. **Tag**: Create a git tag following semantic versioning
4. **Build**: Automated CI builds and publishes artifacts
5. **Announce**: Update documentation and announce the release

## Debugging and Troubleshooting

### Local Debugging

```bash
# Run with debug logging
make run ARGS="--log-level=debug"

# Use delve for step-through debugging
dlv debug ./cmd/main.go -- --log-level=debug
```

### Common Issues

1. **CRD validation errors**: Check schema definitions and regenerate manifests
2. **Controller startup failures**: Verify RBAC permissions and cluster connectivity
3. **Generator failures**: Check external service connectivity and authentication
4. **Storage issues**: Verify backend configuration and permissions

### Useful Commands

```bash
# Check controller logs
kubectl logs -n fx-system deployment/fx-controller-manager -f

# Debug ExternalSource status
kubectl describe externalsource <name>

# Check generated artifacts
kubectl get externalartifact -o yaml

# View controller metrics
kubectl port-forward -n fx-system svc/fx-controller-metrics 8080:8080
curl http://localhost:8080/metrics
```

This developer documentation provides a comprehensive foundation for extending the ExternalSource Controller. For specific questions or advanced use cases, please refer to the source code or open an issue in the project repository.