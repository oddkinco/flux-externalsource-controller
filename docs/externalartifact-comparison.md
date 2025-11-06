# ExternalArtifact CRD Comparison: Custom vs Official

## Quick Reference

### API Group and Version

| Aspect | Custom (Current) | Official (Target) |
|--------|------------------|-------------------|
| API Group | `source.flux.oddkin.co` | `source.toolkit.fluxcd.io` |
| Version | `v1alpha1` | `v1beta2` (or later) |
| CRD Name | `externalartifacts.source.flux.oddkin.co` | `externalartifacts.source.toolkit.fluxcd.io` |

### Spec Structure

#### Custom ExternalArtifact Spec
```yaml
spec:
  url: string          # Required: Location where artifact can be accessed
  revision: string      # Required: Content-based revision (SHA256)
  metadata:            # Optional: Additional metadata
    key: value
```

#### Official ExternalArtifact Spec
```yaml
spec:
  sourceRef:           # Required: Reference to the source
    kind: string       # Required: Kind of source (e.g., "ExternalSource")
    name: string       # Required: Name of the source
    namespace: string  # Optional: Namespace of the source
```

**Key Difference**: Custom CRD stores artifact data in `spec`, while official CRD only references the source in `spec` and stores artifact data in `status`.

### Status Structure

#### Custom ExternalArtifact Status
```yaml
status:
  conditions: []      # Standard Kubernetes conditions
  observedGeneration: int64
```

#### Official ExternalArtifact Status
```yaml
status:
  artifact:           # Artifact details
    url: string       # Location where artifact can be accessed
    revision: string  # Content-based revision
    digest: string    # Digest of the artifact
    lastUpdateTime: string  # Timestamp of last update
    size: int64       # Size of the artifact in bytes
    metadata: map     # Additional metadata
  conditions: []     # Standard Kubernetes conditions
  observedGeneration: int64
```

**Key Difference**: Official CRD stores all artifact information in `status.artifact`, while custom CRD stores it in `spec`.

## Code Mapping

### Creating an ExternalArtifact

#### Custom (Current Implementation)
```go
newArtifact := &sourcev1alpha1.ExternalArtifact{
    ObjectMeta: metav1.ObjectMeta{
        Name:      artifactName,
        Namespace: externalSource.Namespace,
    },
    Spec: sourcev1alpha1.ExternalArtifactSpec{
        URL:      artifactURL,
        Revision: revision,
        Metadata: metadata,
    },
}
```

#### Official (Target Implementation)
```go
newArtifact := &sourcev1beta2.ExternalArtifact{
    ObjectMeta: metav1.ObjectMeta{
        Name:      artifactName,
        Namespace: externalSource.Namespace,
    },
    Spec: sourcev1beta2.ExternalArtifactSpec{
        SourceRef: sourcev1beta2.LocalArtifactReference{
            Kind:      "ExternalSource",
            Name:      externalSource.Name,
            Namespace: externalSource.Namespace,
        },
    },
    Status: sourcev1beta2.ExternalArtifactStatus{
        Artifact: &sourcev1beta2.Artifact{
            URL:            artifactURL,
            Revision:       revision,
            Digest:         revision, // May need proper digest calculation
            LastUpdateTime: metav1.Now(),
            Size:           artifactSize, // If available
            Metadata:       metadata,
        },
    },
}
```

### Updating an ExternalArtifact

#### Custom (Current Implementation)
```go
if existingArtifact.Spec.URL != artifactURL {
    existingArtifact.Spec.URL = artifactURL
    needsUpdate = true
}
if existingArtifact.Spec.Revision != revision {
    existingArtifact.Spec.Revision = revision
    needsUpdate = true
}
if needsUpdate {
    r.Update(ctx, existingArtifact)
}
```

#### Official (Target Implementation)
```go
needsUpdate := false
if existingArtifact.Status.Artifact == nil ||
   existingArtifact.Status.Artifact.URL != artifactURL ||
   existingArtifact.Status.Artifact.Revision != revision {
    existingArtifact.Status.Artifact = &sourcev1beta2.Artifact{
        URL:            artifactURL,
        Revision:       revision,
        Digest:         revision,
        LastUpdateTime: metav1.Now(),
        Metadata:       metadata,
    }
    needsUpdate = true
}
if needsUpdate {
    r.Status().Update(ctx, existingArtifact) // Use Status().Update() for status changes
}
```

### Reading Artifact Information

#### Custom (Current)
```go
artifactURL := existingArtifact.Spec.URL
revision := existingArtifact.Spec.Revision
metadata := existingArtifact.Spec.Metadata
```

#### Official (Target)
```go
if existingArtifact.Status.Artifact != nil {
    artifactURL := existingArtifact.Status.Artifact.URL
    revision := existingArtifact.Status.Artifact.Revision
    digest := existingArtifact.Status.Artifact.Digest
    metadata := existingArtifact.Status.Artifact.Metadata
}
```

## Manifest Examples

### Custom ExternalArtifact Manifest
```yaml
apiVersion: source.flux.oddkin.co/v1alpha1
kind: ExternalArtifact
metadata:
  name: app-config-source
  namespace: default
spec:
  url: https://storage.example.com/artifacts/config-abc123.tar.gz
  revision: abc123def456789...
  metadata:
    source: externalsource-sample
    contentType: application/json
status:
  conditions:
  - type: Ready
    status: "True"
    lastTransitionTime: "2025-01-15T10:30:00Z"
  observedGeneration: 1
```

### Official ExternalArtifact Manifest
```yaml
apiVersion: source.toolkit.fluxcd.io/v1beta2
kind: ExternalArtifact
metadata:
  name: app-config-source
  namespace: default
spec:
  sourceRef:
    kind: ExternalSource
    name: app-config-source
    namespace: default
status:
  artifact:
    url: https://storage.example.com/artifacts/config-abc123.tar.gz
    revision: abc123def456789...
    digest: sha256:abc123def456789...
    lastUpdateTime: "2025-01-15T10:30:00Z"
    size: 1024
    metadata:
      source: externalsource-sample
      contentType: application/json
  conditions:
  - type: Ready
    status: "True"
    lastTransitionTime: "2025-01-15T10:30:00Z"
  observedGeneration: 1
```

## Flux Kustomization Reference

### Using Custom ExternalArtifact
```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: myapp
spec:
  sourceRef:
    kind: ExternalArtifact
    name: app-config-source
    namespace: default
```

### Using Official ExternalArtifact
```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: myapp
spec:
  sourceRef:
    kind: ExternalArtifact
    name: app-config-source
    namespace: default
```

**Note**: The Kustomization reference should remain the same, but the ExternalArtifact resource itself will have a different API version.

## RBAC Differences

### Custom RBAC
```yaml
- apiGroups: ["source.flux.oddkin.co"]
  resources: ["externalartifacts"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["source.flux.oddkin.co"]
  resources: ["externalartifacts/status"]
  verbs: ["get", "update", "patch"]
```

### Official RBAC
```yaml
- apiGroups: ["source.toolkit.fluxcd.io"]
  resources: ["externalartifacts"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["source.toolkit.fluxcd.io"]
  resources: ["externalartifacts/status"]
  verbs: ["get", "update", "patch"]
```

## Key Migration Points

1. **Spec → Status**: Move artifact data from `spec` to `status.artifact`
2. **Add sourceRef**: Add `spec.sourceRef` pointing to ExternalSource
3. **API Group**: Change from `source.flux.oddkin.co` to `source.toolkit.fluxcd.io`
4. **Version**: Change from `v1alpha1` to `v1beta2` (or appropriate version)
5. **Update Logic**: Use `Status().Update()` for status changes instead of `Update()`
6. **Additional Fields**: Handle new fields like `digest`, `size`, `lastUpdateTime`
7. **RBAC**: Update all RBAC references to new API group

## Testing Considerations

When testing the migration:

1. **Verify Status Updates**: Ensure `Status().Update()` works correctly
2. **Check Owner References**: Verify owner references work across API groups
3. **Test Flux Consumption**: Ensure Flux Kustomization can consume the official CRD
4. **Validate Fields**: Verify all required fields are set correctly
5. **Test Updates**: Ensure artifact updates trigger proper reconciliation

