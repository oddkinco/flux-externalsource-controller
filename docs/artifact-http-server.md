# HTTP Artifact Server

## Overview

The HTTP Artifact Server enables the ExternalSource controller to serve artifacts stored in memory or PVC backends through an HTTP endpoint within the cluster. This allows artifacts to be accessed via cluster-internal URLs, making them compatible with Flux and other Kubernetes-native tools.

**Key Features:**
- Serves memory and PVC backend artifacts via HTTP
- StatefulSet architecture for stable pod identity
- Pod-specific URLs for per-pod storage isolation
- Headless service for direct pod access
- Optional ClusterIP service for load-balanced access
- Cluster-internal access using Kubernetes service DNS
- No authentication required (internal-only)
- Configurable port (default: 8080)
- Graceful shutdown support
- PVC support for persistent artifact storage
- S3 backend unchanged (continues serving directly from S3)

## Architecture

### StatefulSet Architecture

The controller uses a StatefulSet deployment pattern for memory and PVC storage backends to provide:

1. **Stable Pod Identity**: Each pod has a predictable name (e.g., `controller-manager-0`, `controller-manager-1`)
2. **Per-Pod Storage**: Each replica has its own isolated storage (in-memory or PVC)
3. **Pod-Specific URLs**: Artifacts are served via pod-specific URLs that include the pod name
4. **Headless Service**: Direct pod-to-pod DNS resolution for accessing artifacts from specific pods

**URL Pattern:**
```
http://{pod-name}.{headless-service}.{namespace}.svc.cluster.local:{port}/{artifact-key}
```

Example:
```
http://controller-manager-0.externalsource-artifacts.flux-system.svc.cluster.local:8080/default/my-source/rev123.tar.gz
```

### Storage Backend Comparison

| Backend | Persistence | Multi-Replica | URL Pattern | Use Case |
|---------|-------------|---------------|-------------|----------|
| Memory | No | Yes (per-pod) | Pod-specific | Development/testing |
| PVC | Yes | Yes (per-pod PVC) | Pod-specific | Production (persistent) |
| S3 | Yes | Yes (shared) | Direct S3 | Production (shared storage) |

### High-Level Overview

```mermaid
graph TB
    subgraph cluster["Kubernetes Cluster"]
        subgraph pod["ExternalSource Controller Pod"]
            subgraph container["Manager Container"]
                reconciler["ExternalSource Reconciler<br/>- Fetches external data<br/>- Packages to tar.gz<br/>- Stores in backend"]
                server["Artifact HTTP Server<br/>- Port: 8080<br/>- GET /artifacts<br/>- No auth required"]
                reconciler --> backend
                server --> backend
                backend["Memory Backend<br/>(Shared State)<br/>- In-memory map<br/>- Non-persistent<br/>- Base URL set"]
            end
        end
        
        service["Service: externalsource-artifacts<br/>- Type: Headless<br/>- Port: 8080<br/>- DNS: http://externalsource-artifacts<br/>.namespace.svc.cluster.local:8080"]
        
        consumers["Flux Kustomization / Other Consumers<br/>- Fetch artifacts via HTTP GET<br/>- URL from ExternalSource status.artifact.url"]
        
        pod -->|"Exposed via"| service
        service -->|"Accessible by"| consumers
    end
    
    style cluster fill:#e8f4f8
    style pod fill:#fff4e6
    style container fill:#f0f0f0
    style backend fill:#d4edda
    style service fill:#cce5ff
    style consumers fill:#e2e3e5
```

### Storage Backend Decision Flow

```mermaid
flowchart TD
    Start["Storage Backend Configuration"] --> Decision{"Backend Type?"}
    
    Decision -->|Memory| Memory["Memory Backend"]
    Decision -->|S3| S3["S3 Backend"]
    
    Memory --> MemoryDetails["Memory Backend<br/>✓ Uses baseURL if provided<br/>✓ Artifacts served via HTTP<br/>✓ URL format:<br/>http://service.ns:8080/<br/>artifacts/ns/name/rev"]
    
    S3 --> S3Details["S3 Backend<br/>✓ Direct S3 URLs<br/>✓ No controller proxy<br/>✓ URL format:<br/>https://s3.../bucket/key"]
    
    style Start fill:#e8f4f8
    style Decision fill:#fff3cd
    style Memory fill:#d4edda
    style S3 fill:#d4edda
    style MemoryDetails fill:#d1ecf1
    style S3Details fill:#d1ecf1
```

### Request Flow for Artifact Retrieval

```mermaid
sequenceDiagram
    participant External as External Source
    participant Controller as ExternalSource Controller
    participant Backend as Memory Backend
    participant Status as ExternalSource Status
    participant Consumer as Flux Kustomization
    participant Server as Artifact HTTP Server

    External->>Controller: 1. Fetch data from external URL
    Controller->>Controller: 2. Package data as tar.gz
    Controller->>Backend: 3. Store artifact<br/>(Key: artifacts/{ns}/{name}/{rev}.tar.gz)
    Backend-->>Controller: Storage complete
    Controller->>Controller: 4. Generate URL<br/>(http://service.ns.svc.cluster.local:8080/{key})
    Controller->>Status: 5. Update status.artifact.url
    Consumer->>Status: 6. Read status.artifact.url
    Consumer->>Server: 7. GET request to artifact server
    Server->>Backend: 8. Retrieve artifact by key
    Backend-->>Server: Return artifact data
    Server-->>Consumer: 9. Serve artifact (application/gzip)
```

## Implementation Details

### Components Modified

#### 1. Configuration (`internal/config/config.go`)
- Added `ArtifactServerConfig` struct with fields: `Enabled`, `Port`, `ServiceName`, `ServiceNamespace`
- Added `ArtifactServer` field to main `Config` struct
- Set defaults in `DefaultConfig()`: Port 8080, ServiceName "externalsource-artifacts"
- Added `loadArtifactServerFromEnv()` to load configuration from environment variables
- Added validation for artifact server configuration

#### 2. Storage Backend Interface (`internal/storage/interface.go`)
- Added `Retrieve(ctx context.Context, key string) ([]byte, error)` method
- Provides a generic way for HTTP server to retrieve artifacts

#### 3. Memory Backend (`internal/storage/memory.go`)
- Added `baseURL` field to `MemoryBackend` struct
- Updated `NewMemoryBackend()` to accept optional `baseURL` parameter
- Modified `Store()` and `GetURL()` to return baseURL-based URLs when baseURL is set
- Added `Retrieve()` method to retrieve artifacts by key
- Maintained backward compatibility: fallback to `memory://localhost/{key}` if baseURL is empty

#### 4. S3 Backend (`internal/storage/s3.go`)
- Added `Retrieve()` method that returns an error indicating S3 artifacts are accessed directly
- S3 backend continues serving artifacts directly from S3

#### 5. HTTP Artifact Server (`internal/artifact/server.go`)
- Created `Server` struct with storage backend reference
- Implemented HTTP handler `serveArtifact` that:
  - Extracts artifact key from URL path
  - Retrieves data from storage backend using `Retrieve()` method
  - Sets appropriate headers (Content-Type: application/gzip, Content-Disposition, Cache-Control)
  - Serves the artifact bytes with proper error handling (404 if not found)
- Implemented `Start(ctx context.Context) error` to start HTTP server in goroutine
- Implemented `Shutdown(ctx context.Context) error` for graceful shutdown

#### 6. Controller (`internal/controller/externalsource_controller.go`)
- Added `StorageBackend` field to `ExternalSourceReconciler` to allow external injection
- Updated `SetupWithManager()` to:
  - Use externally provided storage backend if available
  - Build base URL for memory backend when artifact server is enabled
  - Format: `http://{serviceName}.{namespace}.svc.cluster.local:{port}`

#### 7. Main Application (`cmd/main.go`)
- Added command-line flags:
  - `--artifact-server-port` (default 8080)
  - `--artifact-server-enabled` (default true)
- Added storage backend creation in main to share between controller and artifact server
- Added artifact server startup logic:
  - Only starts if storage backend is "memory" and artifact server is enabled
  - Uses shared storage backend instance
  - Starts server in goroutine before manager starts
  - Ensures graceful shutdown when context is cancelled

#### 8. Kubernetes Manifests

**Service (`config/manager/artifact-service.yaml`):**
- Created headless Service resource named `externalsource-artifacts`
- Type: ClusterIP (internal cluster access only)
- Exposes port 8080 (named "artifacts")
- Selector matches controller pods

**Deployment (`config/manager/manager.yaml`):**
- Added container port 8080 to manager container
- Added environment variables:
  - `ARTIFACT_SERVER_PORT` (default "8080")
  - `ARTIFACT_SERVER_ENABLED` (default "true")
  - `POD_NAMESPACE` (from fieldRef: metadata.namespace)
  - `SERVICE_NAME` (value "externalsource-artifacts")

**Kustomization (`config/manager/kustomization.yaml`):**
- Added `artifact-service.yaml` to resources list

### URL Format

#### Memory and PVC Backends (Pod-Specific)

Artifacts are accessible via pod-specific URLs:
```
http://{pod-name}.{headless-service}.{namespace}.svc.cluster.local:{port}/{artifact-key}
```

Examples:
```
# Direct access to pod 0
http://controller-manager-0.externalsource-artifacts.flux-system.svc.cluster.local:8080/default/my-source/rev123.tar.gz

# Direct access to pod 1
http://controller-manager-1.externalsource-artifacts.flux-system.svc.cluster.local:8080/default/my-source/rev456.tar.gz

# Load-balanced access (via regular ClusterIP service)
http://externalsource-artifacts.flux-system.svc.cluster.local:8080/default/my-source/rev123.tar.gz
```

#### S3 Backend (Direct)

S3 artifacts are accessed directly:
```
https://{s3-endpoint}/{bucket}/{artifact-key}
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `STORAGE_BACKEND` | `memory` | Storage backend type (`memory`, `pvc`, or `s3`) |
| `PVC_STORAGE_PATH` | `/data/artifacts` | Path for PVC storage (when backend is `pvc`) |
| `ARTIFACT_SERVER_ENABLED` | `true` | Enable artifact HTTP server |
| `ARTIFACT_SERVER_PORT` | `8080` | Port for artifact HTTP server |
| `POD_NAMESPACE` | - | Namespace where controller is deployed |
| `POD_NAME` | - | Name of the controller pod (required for memory/PVC backends) |
| `SERVICE_NAME` | `externalsource-artifacts` | Service name for artifact server |

### Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--artifact-server-port` | `8080` | Port for artifact HTTP server |
| `--artifact-server-enabled` | `true` | Enable artifact HTTP server |

## PVC Storage Backend

### Overview

The PVC storage backend provides persistent artifact storage using Kubernetes Persistent Volume Claims. Each StatefulSet replica gets its own PVC, providing isolated, persistent storage that survives pod restarts.

### Configuration

Enable PVC backend using the provided Kustomize patch:

```bash
# Apply the PVC patch
kubectl apply -k config/manager -k config/manager/manager-pvc-patch.yaml
```

Or manually configure:

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
      storageClassName: standard  # Use your storage class
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

### Helm Chart Configuration

Using Helm:

```yaml
# values.yaml
controller:
  storage:
    backend: pvc
    pvc:
      enabled: true
      storageClass: "standard"
      size: 10Gi
      path: "/data/artifacts"
```

Deploy:

```bash
helm install flux-externalsource-controller ./charts/flux-externalsource-controller \
  --namespace flux-system \
  --create-namespace \
  --values values.yaml
```

### Benefits

- **Persistence**: Artifacts survive pod restarts
- **Per-Pod Isolation**: Each replica has its own storage
- **Production Ready**: Suitable for production workloads
- **Scalability**: Supports multiple replicas with independent storage

### Considerations

- Requires a storage class that supports `ReadWriteOnce` access mode
- Each replica requires its own PVC (storage costs scale with replicas)
- PVC cleanup requires manual deletion of PVCs when scaling down

## Testing

### Unit Tests

All unit tests have been implemented and pass:

```bash
# Test storage backends
go test ./internal/storage/... -v

# Test artifact manager and server
go test ./internal/artifact/... -v

# Test config package
go test ./internal/config/... -v

# Run all tests
go test ./... -short
```

### Build and Deploy

#### 1. Build the Controller Image

```bash
# Build the controller
make docker-build IMG=your-registry/flux-externalsource-controller:dev

# Push to registry
make docker-push IMG=your-registry/flux-externalsource-controller:dev
```

#### 2. Deploy to Kubernetes

```bash
# Deploy with default configuration (memory backend)
make deploy IMG=your-registry/flux-externalsource-controller:dev

# Or deploy with kustomize directly
cd config/manager
kustomize edit set image controller=your-registry/flux-externalsource-controller:dev
cd ../..
kubectl apply -k config/default
```

### Verification Steps

#### 1. Verify Controller is Running

```bash
# Check controller pod
kubectl get pods -n flux-system -l control-plane=controller-manager

# Check controller logs
kubectl logs -n flux-system -l control-plane=controller-manager -c manager

# Look for log message:
# "Starting artifact HTTP server for memory backend"
```

#### 2. Verify Artifact Service is Created

```bash
# Check service
kubectl get svc -n flux-system externalsource-artifacts

# Expected output:
# NAME                                        TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)    AGE
# externalsource-artifacts   ClusterIP   None           <none>        8080/TCP   1m
```

#### 3. Create Test ExternalSource

```bash
cat <<EOF | kubectl apply -f -
apiVersion: source.flux.oddkin.co/v1alpha1
kind: ExternalSource
metadata:
  name: test-http-source
  namespace: default
spec:
  interval: 5m
  destinationPath: config.json
  generator:
    type: http
    http:
      url: https://api.github.com/repos/kubernetes/kubernetes/releases/latest
      method: GET
EOF
```

#### 4. Verify Artifact URL

```bash
# Wait for reconciliation
kubectl wait --for=condition=Ready externalsource/test-http-source -n default --timeout=2m

# Check the artifact URL
kubectl get externalsource test-http-source -n default -o jsonpath='{.status.artifact.url}'

# Expected format:
# http://externalsource-artifacts.flux-system.svc.cluster.local:8080/artifacts/default/test-http-source/<revision>.tar.gz
```

#### 5. Test Artifact Retrieval

Create a test pod to fetch the artifact:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: artifact-test
  namespace: default
spec:
  containers:
  - name: curl
    image: curlimages/curl:latest
    command: ['sleep', '3600']
  restartPolicy: Never
EOF

# Wait for pod to be ready
kubectl wait --for=condition=Ready pod/artifact-test -n default --timeout=1m

# Get the artifact URL
ARTIFACT_URL=$(kubectl get externalsource test-http-source -n default -o jsonpath='{.status.artifact.url}')

# Fetch the artifact
kubectl exec -n default artifact-test -- curl -s -o /tmp/artifact.tar.gz "$ARTIFACT_URL"

# Check the result
kubectl exec -n default artifact-test -- ls -lh /tmp/artifact.tar.gz

# Verify it's a valid tar.gz
kubectl exec -n default artifact-test -- tar -tzf /tmp/artifact.tar.gz

# Clean up
kubectl delete pod artifact-test -n default
```

### Integration Test with Flux Kustomization

#### 1. Create a Flux Kustomization

```bash
cat <<EOF | kubectl apply -f -
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: test-external-source
  namespace: flux-system
spec:
  interval: 10m
  sourceRef:
    kind: ExternalSource
    name: test-http-source
    namespace: default
  path: ./
  prune: true
  targetNamespace: default
EOF
```

#### 2. Monitor Reconciliation

```bash
# Watch the kustomization
kubectl get kustomization -n flux-system test-external-source -w

# Check events
kubectl describe kustomization -n flux-system test-external-source

# Check controller logs
kubectl logs -n flux-system -l control-plane=controller-manager -c manager --tail=50
```

## Troubleshooting

### Issue: Artifact Server Not Starting

**Check:**
```bash
# Verify storage backend configuration
kubectl get deployment -n flux-system controller-manager -o yaml | grep -A 5 STORAGE_BACKEND

# Should show:
# - name: STORAGE_BACKEND
#   value: "memory"

# Check artifact server configuration
kubectl get deployment -n flux-system controller-manager -o yaml | grep -A 5 ARTIFACT_SERVER
```

**Solution:**
Ensure environment variables are set correctly in the deployment.

### Issue: 404 Not Found When Fetching Artifact

**Check:**
```bash
# Verify artifact is actually stored
kubectl logs -n flux-system -l control-plane=controller-manager -c manager | grep "Successfully stored artifact"

# Check the exact URL in the status
kubectl get externalsource test-http-source -n default -o yaml | grep -A 10 artifact
```

**Solution:**
- Ensure the ExternalSource reconciliation completed successfully
- Verify the URL format matches the expected pattern
- Check that the artifact server is accessible from within the cluster

### Issue: Connection Refused

**Check:**
```bash
# Verify service endpoints
kubectl get endpoints -n flux-system externalsource-artifacts

# Should show pod IP and port 8080
```

**Solution:**
- Ensure the controller pod is running
- Verify the service selector matches the controller pod labels
- Check network policies if any

## Performance Testing

### Load Test Script

```bash
#!/bin/bash
# test-load.sh

ARTIFACT_URL=$1
CONCURRENT_REQUESTS=${2:-10}
TOTAL_REQUESTS=${3:-100}

echo "Testing $ARTIFACT_URL with $CONCURRENT_REQUESTS concurrent requests, $TOTAL_REQUESTS total"

kubectl run load-test --image=jordi/ab --rm -it --restart=Never -- \
  ab -n $TOTAL_REQUESTS -c $CONCURRENT_REQUESTS $ARTIFACT_URL
```

Usage:
```bash
chmod +x test-load.sh
ARTIFACT_URL=$(kubectl get externalsource test-http-source -n default -o jsonpath='{.status.artifact.url}')
./test-load.sh "$ARTIFACT_URL" 10 100
```

## Production Considerations

1. **Storage Backend**: For production, use S3 backend instead of memory
2. **Resource Limits**: Adjust memory limits based on expected artifact sizes
3. **Network Policies**: Ensure proper network policies allow access to artifact service
4. **Monitoring**: Set up monitoring for artifact server HTTP metrics
5. **High Availability**: Consider running multiple controller replicas (though only leader will serve artifacts)

## Expected Behavior

### Memory Backend
- Artifacts are served through the controller's HTTP server
- URLs use the format: `http://service.namespace.svc.cluster.local:8080/artifacts/...`
- Artifacts are NOT persistent across controller restarts
- Suitable for development and testing

### S3 Backend
- Artifacts are served directly from S3
- URLs use the format: `https://s3.endpoint/bucket/...`
- Artifacts are persistent
- Suitable for production use

## Design Decisions

1. **Memory Backend Only**: Only the memory backend uses the controller HTTP endpoint; S3 backend continues serving artifacts directly from S3
2. **Shared Storage Instance**: The controller and artifact server share the same memory backend instance to ensure consistency
3. **No Authentication**: Artifact downloads require no authentication (cluster-internal only)
4. **Configurable Port**: Port is configurable via flag/env, defaults to 8080
5. **Graceful Shutdown**: Server ensures no dropped connections during shutdown
6. **Backward Compatibility**: Memory backend falls back to `memory://` scheme if no baseURL is provided

## Clean Up

```bash
# Delete test resources
kubectl delete externalsource test-http-source -n default
kubectl delete kustomization test-external-source -n flux-system

# Uninstall controller
make undeploy

# Or with kustomize
kubectl delete -k config/default
```

