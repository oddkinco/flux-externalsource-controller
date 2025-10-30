<!-- c9113c2d-4d62-459e-b483-a5da2f62bf9b 5bb1d086-cbd2-4acc-bc48-2f91b6bb921b -->
# Add HTTP Artifact Server for Memory Backend

## Overview

Implement an HTTP server within the controller to serve artifacts stored in the memory backend. The memory backend will generate URLs pointing to `http://controller-name.namespace.svc.cluster.local:PORT` instead of `memory://localhost`. S3 backend will continue using direct S3 URLs.

## Implementation Steps

### 1. Add Artifact Server Configuration

**File: `internal/config/config.go`**

- Add `ArtifactServer` struct with fields: `Enabled bool`, `Port int`, `ServiceName string`, `ServiceNamespace string`
- Add to main `Config` struct
- Set defaults in `DefaultConfig()`: Port 8080, ServiceName from env or default
- Add environment variable loading in `loadArtifactServerFromEnv()`
- Add validation to ensure port is valid (1-65535)

### 2. Create Artifact HTTP Server

**New file: `internal/artifact/server.go`**

- Create `Server` struct with `storage StorageBackend`, `port int`, `httpServer *http.Server`
- Implement `NewServer(backend StorageBackend, port int) *Server`
- Implement HTTP handler `ServeHTTP` that:
- Extracts artifact key from URL path (e.g., `/artifacts/namespace/name/revision.tar.gz`)
- Retrieves data from storage backend using `GetData()` method (memory backend specific)
- Sets appropriate headers (Content-Type: application/gzip, Content-Disposition)
- Serves the artifact bytes with proper error handling (404 if not found)
- Implement `Start(ctx context.Context) error` to start HTTP server in goroutine
- Implement `Shutdown(ctx context.Context) error` for graceful shutdown

### 3. Update Memory Backend URL Generation

**File: `internal/storage/memory.go`**

- Add `baseURL string` field to `MemoryBackend` struct
- Update `NewMemoryBackend()` signature to accept optional `baseURL string` parameter
- Modify `Store()` to return baseURL-based URL when baseURL is set: `{baseURL}/{key}`
- Modify `GetURL()` similarly to use baseURL when set
- Keep backward compatibility: fallback to `memory://localhost/{key}` if baseURL is empty

### 4. Update Storage Backend Interface

**File: `internal/storage/interface.go`**

- Add `Retrieve(ctx context.Context, key string) ([]byte, error)` method to interface
- This provides a generic way for HTTP server to retrieve artifacts

### 5. Implement Retrieve in Storage Backends

**File: `internal/storage/memory.go`**

- Add `Retrieve(ctx context.Context, key string) ([]byte, error)` method
- Use existing `GetData()` logic, return error if not found

**File: `internal/storage/s3.go`**

- Add `Retrieve(ctx context.Context, key string) ([]byte, error)` method
- Return error indicating S3 artifacts are accessed directly (not through controller)

### 6. Update Controller Setup

**File: `internal/controller/externalsource_controller.go`**

- In `SetupWithManager()`, when creating memory backend, pass the base URL from config
- Base URL format: `http://{serviceName}.{namespace}.svc.cluster.local:{port}`
- Only modify URL generation for memory backend, not S3

### 7. Add Command-Line Flags

**File: `cmd/main.go`**

- Add flags: `--artifact-server-port` (default 8080), `--artifact-server-enabled` (default true for memory)
- Add `ARTIFACT_SERVER_PORT` and `ARTIFACT_SERVER_ENABLED` environment variable support
- Auto-detect service name from `POD_NAME` or use default "flux-externalsource-controller"
- Auto-detect namespace from `POD_NAMESPACE` environment variable

### 8. Start Artifact Server in Main

**File: `cmd/main.go`**

- After controller setup, check if storage backend is "memory" and artifact server is enabled
- Create artifact server with memory backend reference
- Start server in goroutine before `mgr.Start(ctx)`
- Ensure graceful shutdown when context is cancelled

### 9. Create Kubernetes Service Manifest

**New file: `config/manager/artifact-service.yaml`**

- Create Service resource named `flux-externalsource-controller-artifacts`
- Selector matches controller pods: `control-plane: controller-manager`
- Expose port 8080 (named "artifacts") mapping to container port 8080
- Type: ClusterIP (internal cluster access only)
- Add to `config/manager/kustomization.yaml` resources list

### 10. Update Deployment Manifest

**File: `config/manager/manager.yaml`**

- Add container port 8080 to manager container `ports` section
- Add environment variables:
- `ARTIFACT_SERVER_PORT` (default "8080")
- `ARTIFACT_SERVER_ENABLED` (default "true")
- `POD_NAMESPACE` (from fieldRef: metadata.namespace)
- `SERVICE_NAME` (value "flux-externalsource-controller-artifacts")
- Update resources if needed

### 11. Update Tests

**File: `internal/artifact/manager_test.go`**

- Update tests to handle new memory backend URL format
- Mock base URL in tests

**File: `internal/storage/memory_test.go`**

- Add tests for URL generation with baseURL
- Test backward compatibility (empty baseURL uses memory://)

## Key Design Decisions

- Only memory backend uses controller HTTP endpoint
- S3 backend continues serving artifacts directly from S3
- No authentication required for artifact downloads (cluster-internal only)
- Port is configurable via flag/env, defaults to 8080
- Service name follows Kubernetes naming conventions
- Graceful shutdown ensures no dropped connections

### To-dos

- [ ] Add artifact server configuration to Config struct
- [ ] Create HTTP artifact server implementation
- [ ] Update memory backend to use controller base URL
- [ ] Add Retrieve method to storage interface
- [ ] Implement Retrieve in memory and S3 backends
- [ ] Update controller to initialize memory backend with base URL
- [ ] Add command-line flags and env var support for artifact server
- [ ] Start artifact HTTP server in main.go
- [ ] Create Kubernetes Service manifest for artifact server
- [ ] Update deployment with ports and environment variables
- [ ] Update tests for new URL format and server functionality