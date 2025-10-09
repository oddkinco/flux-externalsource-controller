# ExternalSource Controller Deployment

This directory contains Kubernetes manifests for deploying the ExternalSource Controller.

## Directory Structure

- `default/` - Default configuration suitable for most environments
- `development/` - Development configuration with minimal resources
- `production/` - Production configuration with high availability and S3 storage
- `manager/` - Core controller deployment and configuration
- `rbac/` - Role-based access control manifests
- `crd/` - Custom Resource Definitions
- `samples/` - Example ExternalSource and ExternalArtifact resources

## Quick Start

### Development Deployment

For development and testing with in-memory storage:

```bash
kubectl apply -k config/development
```

### Production Deployment

For production with S3 storage:

1. Configure S3 credentials:
   ```bash
   # Create S3 credentials secret
   kubectl create secret generic s3-credentials \
     --from-literal=access-key-id=YOUR_ACCESS_KEY \
     --from-literal=secret-access-key=YOUR_SECRET_KEY \
     -n externalsource-controller-system
   ```

2. Update the S3 configuration in `config/manager/configmap-s3.yaml`

3. Deploy:
   ```bash
   kubectl apply -k config/production
   ```

## Configuration

### Environment Variables

The controller supports configuration via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `STORAGE_BACKEND` | Storage backend type (`memory` or `s3`) | `memory` |
| `S3_ENDPOINT` | S3 endpoint URL | - |
| `S3_BUCKET` | S3 bucket name | - |
| `S3_REGION` | S3 region | `us-east-1` |
| `S3_ACCESS_KEY_ID` | S3 access key ID | - |
| `S3_SECRET_ACCESS_KEY` | S3 secret access key | - |
| `HTTP_TIMEOUT` | HTTP client timeout | `30s` |
| `RETRY_MAX_ATTEMPTS` | Maximum retry attempts | `10` |
| `RETRY_BASE_DELAY` | Base retry delay | `1s` |
| `RETRY_MAX_DELAY` | Maximum retry delay | `5m` |
| `TRANSFORM_TIMEOUT` | CEL transformation timeout | `30s` |
| `METRICS_ENABLED` | Enable Prometheus metrics | `true` |

### ConfigMap Configuration

The controller can also be configured via a ConfigMap. See `config/manager/configmap.yaml` for available options.

### S3 Storage Configuration

For production deployments, configure S3-compatible storage:

1. **AWS S3**: Use standard AWS credentials and region
2. **MinIO**: Set custom endpoint and configure path-style access
3. **Other S3-compatible**: Adjust endpoint and SSL settings as needed

Example S3 configuration:
```yaml
storage.backend: "s3"
storage.s3.endpoint: "https://s3.amazonaws.com"
storage.s3.bucket: "externalsource-artifacts"
storage.s3.region: "us-east-1"
storage.s3.useSSL: "true"
storage.s3.pathStyle: "false"
```

## Security

### Pod Security Standards

The controller is configured to meet the "restricted" Pod Security Standards:

- Runs as non-root user
- Read-only root filesystem
- No privilege escalation
- Drops all capabilities
- Uses seccomp profile

### RBAC

The controller requires the following permissions:

- Read access to Secrets and ConfigMaps
- Full access to ExternalSource and ExternalArtifact resources
- Create/patch access to Events

### Network Policies

Network policies are included to restrict traffic:

- Allow metrics scraping from Prometheus
- Allow API server communication
- Deny other ingress traffic

## Monitoring

### Metrics

The controller exposes Prometheus metrics at `/metrics`:

- `externalsource_reconciliations_total` - Total reconciliations
- `externalsource_reconciliation_duration_seconds` - Reconciliation duration
- `externalsource_source_requests_total` - External source requests
- `externalsource_transformations_total` - Data transformations
- `externalsource_artifacts_total` - Artifact operations

### Health Checks

- Liveness probe: `/healthz` on port 8081
- Readiness probe: `/readyz` on port 8081

## Troubleshooting

### Common Issues

1. **S3 Access Denied**: Verify S3 credentials and bucket permissions
2. **ConfigMap Not Found**: Ensure ConfigMap exists in the same namespace
3. **High Memory Usage**: Adjust transformation memory limits
4. **Network Timeouts**: Increase HTTP timeout values

### Logs

View controller logs:
```bash
kubectl logs -n externalsource-controller-system deployment/controller-manager
```

### Debug Mode

Enable debug logging by setting the `--zap-log-level=debug` flag in the manager args.

## Upgrading

1. Update the controller image tag in your kustomization
2. Apply the updated manifests
3. Monitor the rollout: `kubectl rollout status deployment/controller-manager -n externalsource-controller-system`

## Uninstalling

```bash
kubectl delete -k config/default
```

Or for production:
```bash
kubectl delete -k config/production
```