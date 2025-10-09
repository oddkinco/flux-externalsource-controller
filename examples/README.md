# ExternalSource Examples

This directory contains example ExternalSource manifests for common use cases.

## Basic Examples

- [simple-http.yaml](simple-http.yaml) - Basic HTTP source without authentication
- [authenticated-http.yaml](authenticated-http.yaml) - HTTP source with authentication headers
- [tls-custom-ca.yaml](tls-custom-ca.yaml) - HTTP source with custom CA bundle
- [data-transformation.yaml](data-transformation.yaml) - Source with CEL transformation

## Advanced Examples

- [config-management.yaml](config-management.yaml) - Application configuration management
- [secret-rotation.yaml](secret-rotation.yaml) - Automated secret rotation
- [multi-environment.yaml](multi-environment.yaml) - Environment-specific configuration
- [manifest-generation.yaml](manifest-generation.yaml) - Dynamic Kubernetes manifest generation

## Integration Examples

- [flux-kustomization.yaml](flux-kustomization.yaml) - Consuming ExternalArtifact in Flux
- [monitoring-setup.yaml](monitoring-setup.yaml) - Monitoring and alerting configuration

Each example includes:
- Complete YAML manifests
- Inline documentation
- Expected behavior description
- Troubleshooting tips