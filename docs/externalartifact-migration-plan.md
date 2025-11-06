# Migration Plan: Retire Custom ExternalArtifact CRD and Use Official FluxCD ExternalArtifact

## Overview

This document outlines the plan to migrate from the custom `ExternalArtifact` CRD (`source.flux.oddkin.co/v1alpha1`) to the official FluxCD `ExternalArtifact` CRD (`source.toolkit.fluxcd.io/v1beta2` or later).

## Current State

### Custom ExternalArtifact CRD
- **API Group**: `source.flux.oddkin.co/v1alpha1`
- **Spec Structure**:
  - `url` (string, required): Location where artifact can be accessed
  - `revision` (string, required): Content-based revision (SHA256 hash)
  - `metadata` (map[string]string, optional): Additional metadata
- **Status Structure**:
  - `conditions` ([]Condition): Standard Kubernetes conditions
  - `observedGeneration` (int64): Last observed generation

### Usage in Codebase
1. **Controller**: `internal/controller/externalsource_controller.go`
   - `reconcileExternalArtifact()` function creates/updates ExternalArtifact resources
   - Used as child resources with owner references to ExternalSource
2. **RBAC**: Permissions defined in `config/rbac/` for externalartifacts
3. **Tests**: E2E tests verify ExternalArtifact creation and consumption
4. **Examples**: Flux Kustomization examples reference ExternalArtifact in `sourceRef`

## Target State (Official FluxCD ExternalArtifact)

### Official ExternalArtifact CRD
- **API Group**: `source.toolkit.fluxcd.io/v1beta2` (or later)
- **Spec Structure**:
  - `sourceRef` (required): Reference to the source that produces this artifact
    - `kind`: Kind of the source (e.g., "ExternalSource")
    - `name`: Name of the source
    - `namespace`: Namespace of the source (optional)
- **Status Structure**:
  - `artifact` (Artifact): Contains artifact details
    - `url`: Location where artifact can be accessed
    - `revision`: Content-based revision
    - `digest`: Digest of the artifact
    - `lastUpdateTime`: Timestamp of last update
    - `size`: Size of the artifact
    - `metadata`: Additional metadata
  - `conditions` ([]Condition): Standard Kubernetes conditions
  - `observedGeneration` (int64): Last observed generation

### Key Differences
1. **Spec**: Official CRD uses `sourceRef` instead of direct `url`/`revision` in spec
2. **Status**: Official CRD has `status.artifact` object instead of spec fields
3. **API Group**: Different API group (`source.toolkit.fluxcd.io` vs `source.flux.oddkin.co`)
4. **Version**: Official uses `v1beta2` (or later), custom uses `v1alpha1`

## Migration Steps

### Phase 1: Preparation and Research

1. **Review Official CRD Specification**
   - [ ] Review RFC-0012: https://github.com/fluxcd/flux2/tree/main/rfcs/0012-external-artifact
   - [ ] Review official documentation: https://fluxcd.io/flux/components/source/externalartifacts/
   - [ ] Extract exact CRD schema from Flux v2.7.3+ install.yaml
   - [ ] Identify all required fields and their validation rules

2. **Verify Flux Installation**
   - [ ] Ensure Flux v2.7.0+ is installed in target clusters
   - [ ] Verify official ExternalArtifact CRD is available
   - [ ] Test that Flux controllers can consume official ExternalArtifact

3. **Create Migration Branch**
   - [ ] Create feature branch: `migrate-to-official-externalartifact`
   - [ ] Document current behavior and test coverage

### Phase 2: Code Changes

#### 2.1 Update Type Definitions

1. **Remove Custom ExternalArtifact Types**
   - [ ] Delete `api/v1alpha1/externalartifact_types.go`
   - [ ] Remove ExternalArtifact from `api/v1alpha1/groupversion_info.go` scheme registration
   - [ ] Update `api/v1alpha1/zz_generated.deepcopy.go` (regenerate)

2. **Add Official ExternalArtifact Client**
   - [ ] Add dependency on Flux source-controller API types
   - [ ] Import `sourcev1beta2` or appropriate version from `github.com/fluxcd/source-controller/api`
   - [ ] Update imports in controller code

#### 2.2 Update Controller Logic

1. **Modify `reconcileExternalArtifact()` Function**
   - [ ] Change function signature to use official ExternalArtifact type
   - [ ] Update creation logic:
     - Set `spec.sourceRef.kind = "ExternalSource"`
     - Set `spec.sourceRef.name = externalSource.Name`
     - Set `spec.sourceRef.namespace = externalSource.Namespace` (if needed)
   - [ ] Update status setting:
     - Set `status.artifact.url = artifactURL`
     - Set `status.artifact.revision = revision`
     - Set `status.artifact.digest = revision` (if digest format differs)
     - Set `status.artifact.lastUpdateTime = metav1.Now()`
     - Set `status.artifact.size = artifactSize` (if available)
     - Set `status.artifact.metadata = metadata`
   - [ ] Update comparison logic for detecting changes
   - [ ] Ensure owner reference still works (may need adjustment)

2. **Update Controller Setup**
   - [ ] Update `SetupWithManager()` to watch official ExternalArtifact
   - [ ] Change `Owns()` call to use official type
   - [ ] Update RBAC annotations to reference official API group

#### 2.3 Update RBAC

1. **Update RBAC Permissions**
   - [ ] Update `config/rbac/role.yaml`:
     - Change `source.flux.oddkin.co` to `source.toolkit.fluxcd.io`
     - Change `externalartifacts` resource name (verify exact name)
   - [ ] Update `config/rbac/externalartifact_*.yaml` files
   - [ ] Update Helm chart RBAC templates
   - [ ] Update controller RBAC annotations in code

#### 2.4 Update CRD Generation

1. **Remove Custom CRD**
   - [ ] Delete `config/crd/bases/source.flux.oddkin.co_externalartifacts.yaml`
   - [ ] Remove from `config/crd/kustomization.yaml`
   - [ ] Update `PROJECT` file to remove ExternalArtifact

2. **Update Helm Chart**
   - [ ] Remove ExternalArtifact CRD from `charts/flux-externalsource-controller/templates/crds.yaml`
   - [ ] Update chart documentation
   - [ ] Add note about requiring Flux installation

#### 2.5 Update Examples and Documentation

1. **Update Example Files**
   - [ ] Update `examples/flux-kustomization.yaml`:
     - Change `kind: ExternalArtifact` to use official API version
     - Update `sourceRef` structure if needed
   - [ ] Update `config/samples/source_v1alpha1_externalartifact.yaml`
   - [ ] Update all example files referencing ExternalArtifact

2. **Update Documentation**
   - [ ] Update `README.md` with migration notes
   - [ ] Update `docs/` files referencing ExternalArtifact
   - [ ] Add migration guide for users

### Phase 3: Testing

1. **Unit Tests**
   - [ ] Update `internal/controller/externalsource_controller_test.go`
   - [ ] Update `api/v1alpha1/types_test.go` (remove ExternalArtifact tests)
   - [ ] Verify all tests pass

2. **Integration Tests**
   - [ ] Update `test/e2e/flux_integration_test.go`:
     - Change API version in test manifests
     - Update assertions to check `status.artifact` instead of `spec`
     - Verify Flux Kustomization can consume official ExternalArtifact
   - [ ] Test backward compatibility (if needed)

3. **Manual Testing**
   - [ ] Deploy to test cluster with Flux installed
   - [ ] Create ExternalSource and verify ExternalArtifact creation
   - [ ] Verify Flux Kustomization can consume the artifact
   - [ ] Test update scenarios
   - [ ] Test deletion scenarios

### Phase 4: Migration and Cleanup

1. **Data Migration** (if needed)
   - [ ] Create migration script to convert existing custom ExternalArtifact resources
   - [ ] Document migration process for users
   - [ ] Consider backward compatibility period

2. **Remove Deprecated Code**
   - [ ] Remove all references to custom ExternalArtifact
   - [ ] Clean up unused imports
   - [ ] Update go.mod if dependencies changed

3. **Update Dependencies**
   - [ ] Add Flux source-controller API dependency
   - [ ] Update `go.mod` and `go.sum`
   - [ ] Verify compatibility with Kubernetes versions

### Phase 5: Documentation and Release

1. **Update Documentation**
   - [ ] Write migration guide for users
   - [ ] Update CHANGELOG with breaking changes
   - [ ] Update version compatibility matrix
   - [ ] Document Flux version requirements

2. **Release Planning**
   - [ ] Version bump (major version if breaking change)
   - [ ] Create release notes highlighting migration requirements
   - [ ] Plan deprecation timeline for custom CRD

## Implementation Details

### Code Changes Summary

#### Controller Changes (`internal/controller/externalsource_controller.go`)

```go
// Before:
import sourcev1alpha1 "github.com/oddkinco/flux-externalsource-controller/api/v1alpha1"

existingArtifact := &sourcev1alpha1.ExternalArtifact{}
newArtifact := &sourcev1alpha1.ExternalArtifact{
    Spec: sourcev1alpha1.ExternalArtifactSpec{
        URL:      artifactURL,
        Revision: revision,
        Metadata: metadata,
    },
}

// After:
import sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"

existingArtifact := &sourcev1beta2.ExternalArtifact{}
newArtifact := &sourcev1beta2.ExternalArtifact{
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
            Digest:         revision, // or calculate proper digest
            LastUpdateTime: metav1.Now(),
            Metadata:       metadata,
        },
    },
}
```

#### RBAC Changes

```yaml
# Before:
- apiGroups: ["source.flux.oddkin.co"]
  resources: ["externalartifacts"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

# After:
- apiGroups: ["source.toolkit.fluxcd.io"]
  resources: ["externalartifacts"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

#### Example Manifest Changes

```yaml
# Before:
apiVersion: source.flux.oddkin.co/v1alpha1
kind: ExternalArtifact
metadata:
  name: app-config-source
spec:
  url: https://storage.example.com/artifacts/config-abc123.tar.gz
  revision: abc123def456

# After:
apiVersion: source.toolkit.fluxcd.io/v1beta2
kind: ExternalArtifact
metadata:
  name: app-config-source
spec:
  sourceRef:
    kind: ExternalSource
    name: app-config-source
status:
  artifact:
    url: https://storage.example.com/artifacts/config-abc123.tar.gz
    revision: abc123def456
    lastUpdateTime: "2025-01-15T10:30:00Z"
```

## Risks and Mitigations

### Risk 1: Breaking Changes for Existing Users
- **Mitigation**: 
  - Major version bump
  - Clear migration documentation
  - Consider providing migration script/tool

### Risk 2: Flux Version Compatibility
- **Mitigation**:
  - Document minimum Flux version requirement
  - Test with multiple Flux versions
  - Provide compatibility matrix

### Risk 3: API Group/Version Differences
- **Mitigation**:
  - Thorough testing of API interactions
  - Verify all fields are correctly mapped
  - Test with actual Flux controllers

### Risk 4: Owner Reference Issues
- **Mitigation**:
  - Test that owner references work across API groups
  - Verify garbage collection behavior
  - Consider using finalizers if needed

## Testing Checklist

- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] E2E tests pass with official CRD
- [ ] Flux Kustomization can consume ExternalArtifact
- [ ] Artifact updates work correctly
- [ ] Owner references work correctly
- [ ] Garbage collection works on ExternalSource deletion
- [ ] RBAC permissions are correct
- [ ] Examples work with official CRD
- [ ] Documentation is accurate

## Rollback Plan

If issues are discovered:

1. Revert to previous version
2. Restore custom CRD manifests
3. Document issues encountered
4. Plan fixes for next attempt

## Timeline Estimate

- **Phase 1 (Preparation)**: 1-2 days
- **Phase 2 (Code Changes)**: 3-5 days
- **Phase 3 (Testing)**: 2-3 days
- **Phase 4 (Migration)**: 1-2 days
- **Phase 5 (Documentation)**: 1-2 days

**Total**: ~2 weeks

## Dependencies

- Flux v2.7.0+ installed in target clusters
- Access to Flux source-controller API types
- Test clusters with Flux installed
- User communication about breaking changes

## References

- [RFC-0012: External Artifacts](https://github.com/fluxcd/flux2/tree/main/rfcs/0012-external-artifact)
- [Flux ExternalArtifacts Documentation](https://fluxcd.io/flux/components/source/externalartifacts/)
- [Flux Installation](https://fluxcd.io/flux/installation/)
- [Flux v2.7.3 Install Manifest](https://github.com/fluxcd/flux2/releases/download/v2.7.3/install.yaml)

