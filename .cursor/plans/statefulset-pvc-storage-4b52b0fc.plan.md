<!-- 4b52b0fc-9d45-478c-b358-f9374b1707a5 37e8197a-f809-40e1-96e1-7d225533075f -->
# StatefulSet with PVC Storage Backend

## Problem Summary

The controller currently:

- Uses a Deployment with ClusterIP service
- Serves HTTP artifacts on port 8080 from memory storage (per-pod, non-persistent)
- Helm chart is missing the artifact service (though install YAML has it)
- URL generation doesn't account for pod identity in multi-replica scenarios
- No persistent volume option for artifact storage

## Solution Overview

Convert to StatefulSet architecture with:

- Headless service for stable pod network identity
- Pod-aware URL generation: `http://{pod-name}.{headless-service}.{namespace}.svc.cluster.local:8080/{path}`
- New PVC storage backend storing artifacts as files on persistent volumes
- Per-pod PVCs via volumeClaimTemplates (each replica has its own storage)
- Regular ClusterIP service for artifact HTTP access
- Updated e2e tests to cover PVC backend

## Implementation Plan

### 1. Create PVC Storage Backend

**New file: `internal/storage/pvc.go`**

Create PVC backend that stores artifacts as individual files:

- Implement `StorageBackend` interface
- Store artifacts in `/data/artifacts/{key}` structure
- Use file locking for concurrent access safety
- Support List, Store, Delete, Retrieve, GetURL operations
- Handle directory creation for nested paths
- Return URLs pointing to pod-specific artifact server endpoint

**New file: `internal/storage/pvc_test.go`**

Add comprehensive tests:

- Test CRUD operations with temp directories
- Test concurrent access
- Test directory structure handling
- Test file cleanup

### 2. Update Storage Configuration

**File: `internal/config/config.go`**

Update `StorageConfig`:

- Add "pvc" as valid backend option (alongside "memory" and "s3")
- Add `PVCConfig` struct with:
                                - `Path` (default: "/data/artifacts")
                                - `Enabled bool`
- Update validation to accept "pvc" backend
- Set defaults for PVC backend in `DefaultConfig()`
- Add environment variable loading for PVC config

### 3. Update URL Generation for Pod Identity

**File: `cmd/main.go`**

Modify storage backend initialization:

- Get pod name from `POD_NAME` environment variable
- Get artifact server port from `controllerConfig.ArtifactServer.Port` (configured via ARTIFACT_SERVER_PORT env var)
- For "memory" backend: build URL as `http://{pod-name}.{headless-service}.{namespace}.svc.cluster.local:{port}` where port is from config
- For "pvc" backend: same pod-specific URL pattern with port from config
- For "s3" backend: keep existing direct S3 URL logic
- Add `POD_NAME` environment variable requirement
- Update service name to use headless service for memory/PVC backends
- Ensure port is dynamically read from config, not hardcoded to 8080

### 4. Convert Deployment to StatefulSet

**File: `config/manager/manager.yaml`**

Replace Deployment with StatefulSet:

- Change `kind: Deployment` to `kind: StatefulSet`
- Add `serviceName: flux-externalsource-controller-artifacts-headless`
- Add `volumeClaimTemplates` section:
  ```yaml
  volumeClaimTemplates:
  - metadata:
      name: artifact-storage
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 10Gi
  ```

- Mount PVC at `/data/artifacts` when storage backend is PVC
- Add `POD_NAME` environment variable from `metadata.name`
- Update artifact service name env var to headless service
- Keep support for memory backend (no volume mount needed)

### 5. Create Headless Service

**New file: `config/manager/artifact-headless-service.yaml`**

Create headless service for StatefulSet:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: artifacts
  namespace: system
spec:
  clusterIP: None  # Headless
  selector:
    control-plane: controller-manager
    app.kubernetes.io/name: flux-externalsource-controller
  ports:
  - name: artifacts
    port: 8080
    targetPort: 8080
  publishNotReadyAddresses: true
```

**Delete: `config/manager/artifact-service.yaml`**

Remove the old regular ClusterIP service. The headless service provides direct pod access which is required for the StatefulSet architecture. The Helm chart will handle service creation conditionally.

**Update: `config/manager/kustomization.yaml`**

Replace `artifact-service.yaml` with `artifact-headless-service.yaml` in resources.

### 6. Update Helm Chart Templates

**File: `charts/flux-externalsource-controller/values.yaml`**

Add PVC storage configuration:

```yaml
controller:
  storage:
    backend: memory  # memory, s3, or pvc
    pvc:
      enabled: false
      storageClass: ""
      size: 10Gi
      path: "/data/artifacts"
```

**File: `charts/flux-externalsource-controller/templates/statefulset.yaml`**

Create new StatefulSet template (rename from deployment.yaml):

- Use StatefulSet instead of Deployment
- Add `serviceName` pointing to headless service
- Add conditional volumeClaimTemplates when PVC backend enabled
- Add volume mount for PVC storage when backend is "pvc"
- Add `POD_NAME` environment variable
- Update artifact service reference to headless service

**Update: `charts/flux-externalsource-controller/templates/service.yaml`**

Add headless service definition conditionally:

- Created when backend is "memory" or "pvc"
- ClusterIP: None
- Port 8080 for artifacts
- `publishNotReadyAddresses: true` for pod DNS records

**File: `charts/flux-externalsource-controller/templates/_helpers.tpl`**

Add helper functions:

- `flux-externalsource-controller.artifactHeadlessServiceName` (returns headless service name)
- `flux-externalsource-controller.useStatefulSet` (returns true if memory or pvc backend)

### 7. Update E2E Tests

**File: `test/e2e/e2e_test.go`**

Add new test case:

```go
It("should successfully reconcile ExternalSource with PVC storage backend", func() {
  // Deploy controller with PVC backend
  // Create ExternalSource
  // Verify artifact is stored in PVC
  // Verify artifact is accessible via URL
  // Test pod restart persistence
  // Clean up
})
```

Test scenarios:

- Artifact creation and retrieval with PVC backend
- Persistence across pod restarts
- Multiple artifacts in same PVC
- URL generation with pod name
- Artifact cleanup

**File: `test/e2e/e2e_suite_test.go`**

Add PVC backend test setup:

- Deploy controller with PVC storage backend configuration
- Create necessary PVC resources
- Verify PVC is bound before running tests

### 8. Update Documentation

**File: `docs/artifact-http-server.md`**

Update documentation:

- Explain StatefulSet architecture
- Document headless service pattern
- Document PVC storage backend
- Update URL format examples with pod names
- Add migration guide from Deployment to StatefulSet

**File: `README.md`**

Update main README:

- Document new PVC storage backend option
- Update configuration examples
- Add note about StatefulSet deployment

**File: `charts/flux-externalsource-controller/README.md`**

Update Helm chart README:

- Document new values for PVC storage
- Add examples for different storage backends
- Document StatefulSet configuration

### 9. Update Deployment Manifests

**File: `config/production/manager-production-patch.yaml`**

Add PVC storage configuration option for production.

**New file: `config/manager/manager-pvc-patch.yaml`**

Create patch for PVC backend:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: controller-manager
spec:
  volumeClaimTemplates:
  - metadata:
      name: artifact-storage
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 10Gi
  template:
    spec:
      containers:
      - name: manager
        env:
        - name: STORAGE_BACKEND
          value: "pvc"
        - name: PVC_STORAGE_PATH
          value: "/data/artifacts"
        volumeMounts:
        - name: artifact-storage
          mountPath: /data/artifacts
```

### 10. Update Integration Tests

**Files: `internal/storage/pvc_test.go`, `internal/controller/externalsource_controller_test.go`**

Add unit tests:

- PVC backend operations
- Controller integration with PVC backend
- URL generation with pod names
- Error handling for PVC operations

### Key Changes Summary

**Storage Backend URLs:**

- Memory: `http://{pod-name}.{headless-svc}.{ns}.svc.cluster.local:8080/{key}`
- PVC: `http://{pod-name}.{headless-svc}.{ns}.svc.cluster.local:8080/{key}`
- S3: `https://{s3-endpoint}/{bucket}/{key}` (unchanged)

**Architecture:**

- Deployment → StatefulSet
- Single ClusterIP service → Headless service (for direct pod access)
- Per-pod storage (memory/PVC) with pod-specific URLs
- Multi-replica support with per-pod PVCs
- Helm chart conditionally creates headless service for memory/PVC backends

**Configuration:**

- New `STORAGE_BACKEND=pvc` option
- New `PVC_STORAGE_PATH` environment variable
- New `POD_NAME` environment variable (required)
- Helm values for PVC size, storageClass, path

### Implementation Notes

**Service Architecture Decision:**
The old regular ClusterIP service (`config/manager/artifact-service.yaml`) was completely removed and replaced with a headless service. This is because the StatefulSet architecture requires direct pod-to-pod communication via pod-specific DNS names to properly isolate per-pod storage. The headless service enables this by creating individual DNS records for each pod (e.g., `controller-manager-0.artifacts.namespace.svc.cluster.local`). The Helm chart now handles service creation conditionally within the `service.yaml` template based on the storage backend type.

### To-dos

- [x] Create PVC storage backend implementation in internal/storage/pvc.go with file-based artifact storage
- [x] Update storage configuration to support 'pvc' backend option with PVCConfig struct
- [x] Update cmd/main.go to generate pod-specific URLs using POD_NAME for memory and PVC backends
- [x] Convert config/manager/manager.yaml from Deployment to StatefulSet with volumeClaimTemplates
- [x] Create headless service manifest for StatefulSet pod discovery and delete old ClusterIP service
- [x] Update Helm chart to use StatefulSet template with PVC support and conditionally create headless service
- [x] Add e2e tests for PVC storage backend covering persistence and pod restarts
- [x] Update documentation for StatefulSet architecture, PVC backend, and new URL patterns