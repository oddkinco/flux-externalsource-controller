<!-- 096b4665-791e-4d3b-b3f6-2ec01e160ef5 c06c2b67-008b-45df-9fd2-1c0efe2fe31c -->
# Replace CEL Transformer with Generic Hook System

## Overview

Replace the CEL-based post-request transformer with a more flexible hook system that supports both pre-request and post-request command execution. Commands run in sidecar containers and are controlled by a whitelist loaded from a mounted file.

## API Changes

### Update ExternalSource CRD (`api/v1alpha1/externalsource_types.go`)

- Remove `Transform` and `TransformSpec` types entirely
- Add `maxRetries` field to `ExternalSourceSpec` (default: 3)
- Add `Hooks` field with `HooksSpec` type containing:
- `preRequest []HookSpec` - hooks to run before HTTP request
- `postRequest []HookSpec` - hooks to run after HTTP request
- Define `HookSpec` type:
- `name string` - unique hook name
- `command string` - command to execute (must be in whitelist)
- `args []string` - arguments to pass to command
- `timeout string` - timeout duration (e.g., "30s")
- `retryPolicy string` - enum: "ignore", "retry", "fail"
- `env []EnvVar` - optional environment variables

### Update Status

- Remove CEL-related status messages
- Track hook execution in conditions

## Hook Execution System

### Create Hook Executor (`internal/hooks/executor.go`)

- Interface `HookExecutor` with method `Execute(ctx, input []byte, hook HookSpec) ([]byte, error)`
- Implement `SidecarExecutor` that communicates with sidecar via HTTP/gRPC
- Support stdin/stdout byte streaming
- Handle timeout enforcement
- Implement retry policy logic (ignore, retry, fail)

### Create Whitelist Manager (`internal/hooks/whitelist.go`)

- Load whitelist from file at startup (path from env var `HOOK_WHITELIST_PATH`, default `/etc/hooks/whitelist.yaml`)
- Whitelist format: YAML with command names and optional allowed arguments patterns
- Validate commands against whitelist before execution
- Provide `IsAllowed(command string, args []string) bool` method

### Sidecar Communication Protocol

- Sidecar listens on localhost:8081 (configurable)
- REST API endpoints:
- `POST /execute` - execute command with request body as stdin
- Request: `{"command": "jq", "args": [".field"], "timeout": "30s", "env": {...}}`
- Response: `{"stdout": "base64...", "stderr": "base64...", "exitCode": 0}`
- Controller validates command against whitelist before sending to sidecar

## Controller Changes

### Update Reconciliation Logic (`internal/controller/externalsource_controller.go`)

- Remove `Transformer` field from `ExternalSourceReconciler`
- Add `HookExecutor` field
- Remove transformation step (lines ~340-359)
- Add pre-request hook execution before `sourceGenerator.Generate()`
- Execute all pre-request hooks in order
- Apply retry logic per hook's retryPolicy
- Allow hooks to modify request config (return modified config as JSON)
- Add post-request hook execution after fetch
- Execute all post-request hooks in order
- Pass response data through hook pipeline
- Apply retry logic per hook's retryPolicy
- Count retries against `maxRetries` aggregate limit
- Update error handling to support hook retry policies
- Remove `TransformingCondition` references
- Add `ExecutingHooksCondition` for observability

### Update Metrics (`internal/metrics/prometheus.go`)

- Remove transformation metrics (`RecordTransformation`)
- Add hook execution metrics:
- `externalsource_hook_execution_total{hook_name, retry_policy, status}`
- `externalsource_hook_execution_duration_seconds{hook_name}`

## Configuration Changes

### Update Config (`internal/config/config.go`)

- Remove `TransformConfig` and related environment loading
- Add `HooksConfig`:
- `WhitelistPath string` - path to whitelist file
- `SidecarEndpoint string` - sidecar communication endpoint
- `DefaultTimeout time.Duration` - default hook timeout
- Remove Transform validation from `Validate()`

### Update Deployment Manifests

- Add sidecar container to `config/manager/manager.yaml`:
- Image: `ghcr.io/oddkinco/hook-executor:latest` (or similar)
- Shared localhost network
- Mount whitelist ConfigMap at `/etc/hooks/whitelist.yaml`
- Create example whitelist ConfigMap in `config/manager/hooks-whitelist.yaml`
- Update RBAC if needed for ConfigMap access

## Removal Tasks

### Delete CEL Components

- Delete `internal/transformer/cel.go`
- Delete `internal/transformer/cel_test.go`
- Delete `internal/transformer/interface.go`
- Remove CEL dependency from `go.mod` (`github.com/google/cel-go`)
- Run `go mod tidy`

### Update Examples

- Remove or update `examples/data-transformation.yaml` to use hooks
- Create new example `examples/hooks-example.yaml` demonstrating pre/post hooks
- Update `examples/README.md` to document hook system

## Testing Updates

### Unit Tests

- Add tests for `internal/hooks/executor_test.go`
- Add tests for `internal/hooks/whitelist_test.go`
- Update controller tests to mock `HookExecutor`
- Remove all CEL/transformer test references

### Integration Tests

- Update e2e tests to include hook execution scenarios
- Test retry policies (ignore, retry, fail)
- Test maxRetries aggregate limit
- Test whitelist enforcement

## Hook Executor Binary

### Create Reference Implementation (`cmd/hook-executor/`)

- Create new Go binary in `cmd/hook-executor/main.go`
- Implement HTTP server listening on configurable port (default 8081)
- REST API endpoints:
- `POST /execute` - execute whitelisted command with stdin/stdout
- `GET /health` - health check endpoint
- Load whitelist from mounted file on startup
- Validate all commands against whitelist before execution
- Execute commands with:
- Timeout enforcement
- Environment variable support
- Stdin/stdout byte streaming
- Proper error handling and exit codes
- Return JSON response with base64-encoded stdout/stderr

### Create Dockerfile (`cmd/hook-executor/Dockerfile`)

- Multi-stage build:
- Build stage: compile Go binary
- Runtime stage: minimal Alpine image
- Install common utilities in runtime image:
- `jq` for JSON processing
- `yq` for YAML processing
- `curl` for HTTP calls
- `bash` for shell scripts
- Copy hook-executor binary
- Set entrypoint to hook-executor
- Expose port 8081

### GitHub Actions Workflow (`.github/workflows/hook-executor.yaml`)

- Trigger on:
- Push to main (with changes to `cmd/hook-executor/`)
- Tagged releases matching `hook-executor-v*`
- Build multi-arch Docker image (amd64, arm64)
- Tag images:
- `ghcr.io/oddkinco/hook-executor:latest` (main branch)
- `ghcr.io/oddkinco/hook-executor:v{version}` (tagged releases)
- `ghcr.io/oddkinco/hook-executor:sha-{commit}` (all builds)
- Push to GitHub Container Registry
- Require GITHUB_TOKEN with packages:write permission

### Example Whitelist (`cmd/hook-executor/examples/whitelist.yaml`)

- Create example whitelist configuration showing:
- Simple commands (jq, yq)
- Commands with argument patterns
- Commands with restricted arguments
- Document whitelist format and syntax

## Documentation Updates

- Update design documents (`.kiro/specs/externalsource-controller/design.md`)
- Remove CEL transformer references
- Add hook system architecture
- Update requirements document
- Update proposal document
- Update README with hook examples
- Add hook development guide

### To-dos

- [ ] Update ExternalSource CRD to remove Transform field and add Hooks and maxRetries fields
- [ ] Create hook executor and whitelist manager interfaces and implementations
- [ ] Update controller reconciliation logic to execute hooks instead of CEL transformation
- [ ] Delete CEL transformer code and remove dependencies
- [ ] Update configuration to remove Transform config and add Hooks config
- [ ] Update metrics to track hook execution instead of transformation
- [ ] Add sidecar container and whitelist ConfigMap to deployment manifests
- [ ] Update and create examples demonstrating the hook system
- [ ] Update unit and integration tests for hook system
- [ ] Update all documentation to reflect hook system instead of CEL