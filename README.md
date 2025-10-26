# ExternalSource Controller

A Kubernetes operator that integrates external, non-Git data sources into GitOps workflows powered by Flux.

## Overview

The ExternalSource Controller is a Kubernetes operator built using the Kubebuilder framework that enables integration of external HTTP-based data sources into Flux GitOps workflows. The controller implements an asynchronous reconciliation pattern to fetch, transform, and package external data as versioned artifacts consumable by other Flux controllers.

### Key Features

- **Modular Source Generators**: Pluggable architecture supporting HTTP sources with easy extensibility for future source types
- **Data Transformation**: Optional CEL-based transformation of fetched data using Common Expression Language
- **Artifact Management**: Automatic packaging and versioning of external data as .tar.gz archives with SHA256 content hashing
- **Flux Integration**: Seamless integration with existing Flux controllers through ExternalArtifact resources
- **Observability**: Comprehensive Prometheus metrics and status reporting for monitoring and troubleshooting
- **Resilience**: Built-in retry logic with exponential backoff and graceful error handling
- **Security**: Support for TLS configuration, custom CA bundles, and authentication via Kubernetes secrets

### Use Cases

- **Configuration Management**: Fetch application configuration from external APIs and deploy via GitOps
- **Secret Rotation**: Automatically retrieve updated secrets from external systems
- **Dynamic Manifests**: Generate Kubernetes manifests from external data sources
- **Multi-Environment Config**: Sync environment-specific configuration from centralized APIs
- **Third-Party Integration**: Integrate with external systems that don't support Git-based workflows

## Quick Start

### Prerequisites

- Kubernetes cluster v1.25+ with admin access
- kubectl configured to access your cluster
- Flux v2 installed in your cluster (for consuming ExternalArtifact resources)

### Installation

#### Option 1: Using Pre-built Images (Recommended)

1. **Install the CRDs:**
   ```bash
   kubectl apply -f https://github.com/oddkinco/flux-externalsource-controller/releases/latest/download/crds.yaml
   ```

2. **Deploy the controller:**
   ```bash
   kubectl apply -f https://github.com/oddkinco/flux-externalsource-controller/releases/latest/download/flux-externalsource-controller.yaml
   ```

#### Option 2: Build from Source

1. **Clone the repository:**
   ```bash
   git clone https://github.com/oddkinco/flux-externalsource-controller.git
   cd flux-externalsource-controller
   ```

2. **Install CRDs:**
   ```bash
   make install
   ```

3. **Build and deploy:**
   ```bash
   make docker-build docker-push IMG=<your-registry>/flux-externalsource-controller:latest
   make deploy IMG=<your-registry>/flux-externalsource-controller:latest
   ```

### Basic Usage

1. **Create an ExternalSource resource:**
   ```yaml
   apiVersion: source.flux.oddkin.co/v1alpha1
   kind: ExternalSource
   metadata:
     name: my-config
     namespace: default
   spec:
     interval: 5m
     generator:
       type: http
       http:
         url: https://api.example.com/config
   ```

2. **Apply the resource:**
   ```bash
   kubectl apply -f externalsource.yaml
   ```

3. **Check the status:**
   ```bash
   kubectl get externalsource my-config -o yaml
   ```

4. **View the created artifact:**
   ```bash
   kubectl get externalartifact
   ```

## Configuration

### ExternalSource Specification

The ExternalSource custom resource supports the following configuration options:

#### Core Fields

- **interval** (required): How often to check for updates (minimum 1m)
- **suspend** (optional): Suspend reconciliation when set to true
- **destinationPath** (optional): Path within the artifact where data should be placed

#### Generator Configuration

Currently supports HTTP generators with the following options:

```yaml
spec:
  generator:
    type: http
    http:
      url: "https://api.example.com/data"          # Required: API endpoint
      method: "GET"                                # Optional: HTTP method (default: GET)
      headersSecretRef:                           # Optional: Authentication headers
        name: "api-credentials"
      caBundleSecretRef:                          # Optional: Custom CA bundle
        name: "ca-bundle"
        key: "ca.crt"
      insecureSkipVerify: false                   # Optional: Skip TLS verification (not recommended)
```

#### Data Transformation

Optional CEL-based transformation of fetched data:

```yaml
spec:
  transform:
    type: cel
    expression: |
      has(data.config) ? data.config : data
```

### Controller Configuration

The controller supports configuration through environment variables:

- **STORAGE_BACKEND**: `s3` or `memory` (default: memory)
- **S3_BUCKET**: S3 bucket name for artifact storage
- **S3_REGION**: S3 region
- **HTTP_TIMEOUT**: HTTP request timeout (default: 30s)
- **TRANSFORM_TIMEOUT**: Transformation timeout (default: 10s)

## Examples

### Simple HTTP Source

Fetch JSON configuration from an API:

```yaml
apiVersion: source.flux.oddkin.co/v1alpha1
kind: ExternalSource
metadata:
  name: app-config
  namespace: default
spec:
  interval: 10m
  destinationPath: config.json
  generator:
    type: http
    http:
      url: https://config-api.example.com/app/config
```

### Authenticated HTTP Source

Fetch data with authentication headers:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: api-token
  namespace: default
type: Opaque
data:
  Authorization: Bearer <base64-encoded-token>
---
apiVersion: source.flux.oddkin.co/v1alpha1
kind: ExternalSource
metadata:
  name: secure-config
  namespace: default
spec:
  interval: 5m
  generator:
    type: http
    http:
      url: https://secure-api.example.com/config
      headersSecretRef:
        name: api-token
```

### Data Transformation

Transform API response before packaging:

```yaml
apiVersion: source.flux.oddkin.co/v1alpha1
kind: ExternalSource
metadata:
  name: transformed-config
  namespace: default
spec:
  interval: 15m
  destinationPath: app-config.yaml
  generator:
    type: http
    http:
      url: https://api.example.com/raw-config
  transform:
    type: cel
    expression: |
      {
        "apiVersion": "v1",
        "kind": "ConfigMap",
        "metadata": {"name": "app-config"},
        "data": {"config.json": string(data)}
      }
```

### Custom TLS Configuration

Use custom CA bundle for TLS verification:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: custom-ca
  namespace: default
type: Opaque
data:
  ca.crt: <base64-encoded-ca-certificate>
---
apiVersion: source.flux.oddkin.co/v1alpha1
kind: ExternalSource
metadata:
  name: tls-config
  namespace: default
spec:
  interval: 30m
  generator:
    type: http
    http:
      url: https://internal-api.company.com/config
      caBundleSecretRef:
        name: custom-ca
        key: ca.crt
```

## Monitoring and Observability

### Status Conditions

ExternalSource resources provide detailed status information:

```bash
kubectl describe externalsource my-config
```

Status conditions include:
- **Ready**: Overall health of the ExternalSource
- **Fetching**: Currently fetching data from external source
- **Transforming**: Currently applying transformations
- **Storing**: Currently storing artifact
- **Stalled**: Reconciliation has been stalled due to errors

### Prometheus Metrics

The controller exposes metrics at `/metrics` endpoint:

- `externalsource_reconcile_total`: Total number of reconciliations
- `externalsource_reconcile_duration_seconds`: Reconciliation duration
- `externalsource_http_request_duration_seconds`: HTTP request latency
- `externalsource_transform_duration_seconds`: Transformation duration

### Logs

View controller logs for detailed troubleshooting:

```bash
kubectl logs -n flux-externalsource-controller-system deployment/flux-externalsource-controller-manager
```

## Flux Integration

### Consuming ExternalArtifacts

Use ExternalArtifact resources in Flux Kustomization:

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: app-deployment
  namespace: flux-system
spec:
  interval: 5m
  sourceRef:
    kind: ExternalArtifact
    name: app-config
    namespace: default
  path: "./"
  prune: true
```

### GitOps Workflow

1. ExternalSource fetches data from external API
2. Controller packages data as versioned artifact
3. ExternalArtifact resource is created/updated
4. Flux Kustomization consumes the artifact
5. Application configuration is deployed to cluster

## Troubleshooting

### Common Issues

**ExternalSource stuck in "Fetching" state:**
- Check network connectivity to external API
- Verify authentication credentials in referenced secrets
- Review controller logs for detailed error messages

**Transformation failures:**
- Validate CEL expression syntax
- Ensure input data format matches expression expectations
- Check transformation timeout settings

**Artifact storage issues:**
- Verify S3 credentials and permissions
- Check storage backend configuration
- Ensure sufficient storage space

### Debug Commands

```bash
# Check ExternalSource status
kubectl get externalsource -A

# View detailed status
kubectl describe externalsource <name>

# Check controller logs
kubectl logs -n flux-externalsource-controller-system deployment/flux-externalsource-controller-manager

# View metrics
kubectl port-forward -n flux-externalsource-controller-system svc/flux-externalsource-controller-metrics 8080:8080
curl http://localhost:8080/metrics
```

## Development

### Prerequisites

- Go 1.24+
- Docker or Podman
- kubectl
- Kind (for local testing)

### Building

```bash
# Build binary
make build

# Run tests
make test

# Build container image
make docker-build IMG=flux-externalsource-controller:dev

# Run locally (requires cluster access)
make run
```

### Testing

```bash
# Unit tests
make test

# End-to-end tests
make test-e2e
```

## Contributing

We welcome pull requests! Please see [Developer Documentation](docs/development.md) for detailed guidance of setting up a development environment.

### Adding New Source Types

The controller is designed for extensibility. To add a new source type:

1. Implement the `SourceGenerator` interface
2. Register the generator with the factory
3. Update the CRD schema
4. Add tests and documentation

See [Extending Source Generators](docs/extending-generators.md) for more information.

## License

Copyright (c) 2025 Odd Kin <oddkin@oddkin.co>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.