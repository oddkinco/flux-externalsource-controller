# flux-externalsource-controller Helm Chart

A Helm chart for deploying the ExternalSource Controller, which integrates external data sources into GitOps workflows powered by Flux.

## Prerequisites

- Kubernetes 1.25+
- Helm 3.8+
- Flux v2 (for consuming ExternalArtifact resources)

## Installation

### Add Helm Repository

```bash
helm repo add flux-externalsource-controller oci://ghcr.io/oddkin/charts
helm repo update
```

### Install Chart

```bash
# Install with default values
helm install flux-externalsource-controller flux-externalsource-controller/flux-externalsource-controller

# Install in custom namespace
helm install flux-externalsource-controller flux-externalsource-controller/flux-externalsource-controller --namespace flux-external-system --create-namespace

# Install with custom values
helm install flux-externalsource-controller flux-externalsource-controller/flux-externalsource-controller -f values.yaml
```

### Install from OCI Registry

```bash
helm install flux-externalsource-controller oci://ghcr.io/oddkin/charts/flux-externalsource-controller --version 0.1.0
```

## Configuration

The following table lists the configurable parameters and their default values.

### Controller Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `controller.replicas` | Number of controller replicas | `1` |
| `controller.logLevel` | Log level (debug, info, warn, error) | `info` |
| `controller.leaderElection` | Enable leader election | `true` |

### Image Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Controller image repository | `ghcr.io/oddkin/flux-externalsource-controller` |
| `image.tag` | Controller image tag | `""` (uses chart appVersion) |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `imagePullSecrets` | Image pull secrets | `[]` |

### Storage Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `controller.storage.backend` | Storage backend (memory or s3) | `memory` |
| `controller.storage.s3.bucket` | S3 bucket name | `""` |
| `controller.storage.s3.region` | S3 region | `""` |
| `controller.storage.s3.endpoint` | S3 endpoint URL | `""` |
| `controller.storage.s3.credentialsSecret.name` | Secret containing S3 credentials | `""` |

### Resource Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `controller.resources.limits.cpu` | CPU limit | `500m` |
| `controller.resources.limits.memory` | Memory limit | `128Mi` |
| `controller.resources.requests.cpu` | CPU request | `10m` |
| `controller.resources.requests.memory` | Memory request | `64Mi` |

### RBAC Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `rbac.create` | Create RBAC resources | `true` |
| `serviceAccount.create` | Create service account | `true` |
| `serviceAccount.name` | Service account name | `""` (generated) |
| `serviceAccount.annotations` | Service account annotations | `{}` |

### Metrics Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `metrics.enabled` | Enable metrics service | `true` |
| `metrics.service.type` | Metrics service type | `ClusterIP` |
| `metrics.service.port` | Metrics service port | `8080` |
| `metrics.serviceMonitor.enabled` | Create ServiceMonitor for Prometheus | `false` |
| `metrics.serviceMonitor.interval` | Scrape interval | `30s` |

### CRD Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `crds.install` | Install CRDs with chart | `true` |
| `crds.keep` | Keep CRDs on chart uninstall | `true` |

## Examples

### Basic Installation

```bash
helm install flux-externalsource-controller oci://ghcr.io/oddkin/charts/flux-externalsource-controller
```

### Installation with S3 Storage

```yaml
# values-s3.yaml
controller:
  storage:
    backend: s3
    s3:
      bucket: my-fx-artifacts
      region: us-west-2
      credentialsSecret:
        name: s3-credentials

# Create S3 credentials secret
kubectl create secret generic s3-credentials \
  --from-literal=access-key=YOUR_ACCESS_KEY \
  --from-literal=secret-key=YOUR_SECRET_KEY

# Install with S3 storage
helm install flux-externalsource-controller oci://ghcr.io/oddkin/charts/flux-externalsource-controller -f values-s3.yaml
```

### Installation with Monitoring

```yaml
# values-monitoring.yaml
metrics:
  enabled: true
  serviceMonitor:
    enabled: true
    namespace: monitoring
    labels:
      release: prometheus

controller:
  logLevel: debug
```

```bash
helm install flux-externalsource-controller oci://ghcr.io/oddkin/charts/flux-externalsource-controller -f values-monitoring.yaml
```

### High Availability Installation

```yaml
# values-ha.yaml
controller:
  replicas: 3
  leaderElection: true
  affinity:
    podAntiAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchExpressions:
            - key: app.kubernetes.io/name
              operator: In
              values:
              - flux-externalsource-controller
          topologyKey: kubernetes.io/hostname

podDisruptionBudget:
  enabled: true
  minAvailable: 2
```

```bash
helm install flux-externalsource-controller oci://ghcr.io/oddkin/charts/flux-externalsource-controller -f values-ha.yaml
```

## Upgrading

### Upgrade to Latest Version

```bash
helm repo update
helm upgrade flux-externalsource-controller flux-externalsource-controller/flux-externalsource-controller
```

### Upgrade with Custom Values

```bash
helm upgrade flux-externalsource-controller flux-externalsource-controller/flux-externalsource-controller -f values.yaml
```

### Upgrade CRDs

CRDs are not automatically upgraded by Helm. To upgrade CRDs:

```bash
# Download and apply latest CRDs
kubectl apply -f https://github.com/oddkin/flux-externalsource-controller/releases/latest/download/crds.yaml
```

## Uninstalling

```bash
# Uninstall the chart
helm uninstall flux-externalsource-controller

# Optionally remove CRDs (this will delete all ExternalSource and ExternalArtifact resources)
kubectl delete crd externalsources.source.flux.oddkin.co externalartifacts.source.flux.oddkin.co
```

## Troubleshooting

### Common Issues

1. **Controller not starting:**
   ```bash
   kubectl logs -n flux-external-system deployment/flux-externalsource-controller-manager
   kubectl describe deployment -n flux-external-system flux-externalsource-controller-manager
   ```

2. **RBAC issues:**
   ```bash
   kubectl auth can-i create externalsources --as=system:serviceaccount:flux-external-system:flux-externalsource-controller-manager
   ```

3. **Storage backend issues:**
   ```bash
   # Check S3 credentials
   kubectl get secret s3-credentials -o yaml
   
   # Test S3 connectivity
   kubectl run -it --rm debug --image=amazon/aws-cli --restart=Never -- s3 ls s3://your-bucket
   ```

### Debug Mode

Enable debug logging:

```yaml
controller:
  logLevel: debug
```

### Health Checks

The controller exposes health check endpoints:

- Liveness: `http://localhost:8081/healthz`
- Readiness: `http://localhost:8081/readyz`
- Metrics: `http://localhost:8080/metrics`

## Development

### Local Development

```bash
# Clone the repository
git clone https://github.com/oddkin/flux-externalsource-controller.git
cd flux-externalsource-controller

# Install chart locally
helm install flux-externalsource-controller ./charts/flux-externalsource-controller --set image.tag=dev

# Test chart templates
helm template flux-externalsource-controller ./charts/flux-externalsource-controller --debug
```

### Contributing

1. Make changes to chart templates or values
2. Update version in `Chart.yaml`
3. Test changes with `helm lint` and `helm template`
4. Submit pull request

## Support

- GitHub Issues: https://github.com/oddkin/flux-externalsource-controller/issues
- Documentation: https://github.com/oddkin/flux-externalsource-controller/tree/main/docs
- Discussions: https://github.com/oddkin/flux-externalsource-controller/discussions