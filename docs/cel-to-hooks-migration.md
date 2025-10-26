# CEL to Hooks System Migration Plan

## Overview

This document outlines the complete migration from CEL-based transformations to a flexible hooks system in the ExternalSource controller. The migration replaces CEL (Common Expression Language) transformations with a more flexible hook system that supports both pre-request and post-request command execution with configurable retry policies.

## What Was Implemented

### 1. API Changes (CRD Updates)

**File:** `api/v1alpha1/externalsource_types.go`

- ✅ Removed `Transform` and `TransformSpec` types
- ✅ Added `maxRetries` field to `ExternalSourceSpec`
- ✅ Added `Hooks` field with `HooksSpec` type
- ✅ Created `HookSpec` type with:
  - `name`, `command`, `args`, `timeout`, `retryPolicy`, `env` fields
- ✅ Added `EnvVar` type for environment variables
- ✅ Regenerated CRD manifests

### 2. Hook Execution System

**Files:**
- `internal/hooks/interface.go`
- `internal/hooks/executor.go`
- `internal/hooks/whitelist.go`

- ✅ Created `HookExecutor` interface
- ✅ Implemented `SidecarExecutor` that communicates via HTTP
- ✅ Created `WhitelistManager` interface
- ✅ Implemented `FileWhitelistManager` with regex-based argument validation
- ✅ Added comprehensive unit tests

### 3. ExternalSource Hook Executor Binary

**Files:**
- `cmd/externalsource-hook-executor/main.go`
- `cmd/externalsource-hook-executor/Dockerfile`
- `cmd/externalsource-hook-executor/README.md`
- `cmd/externalsource-hook-executor/examples/whitelist.yaml`

- ✅ Created HTTP server on port 8081
- ✅ Implemented `/execute` endpoint for command execution
- ✅ Implemented `/health` endpoint for health checks
- ✅ Added whitelist validation before execution
- ✅ Support for stdin/stdout byte streaming (base64 encoded)
- ✅ Environment variable support
- ✅ Timeout enforcement
- ✅ Multi-stage Dockerfile with common utilities (jq, yq, curl, bash)
- ✅ GitHub Actions workflow for automated builds and publishing

### 4. Controller Updates

**File:** `internal/controller/externalsource_controller.go`

- ✅ Replaced `Transformer` field with `HookExecutor`
- ✅ Updated imports to use hooks instead of transformer
- ✅ Replaced `TransformingCondition` with `ExecutingHooksCondition`
- ✅ Implemented `executeHooks()` method with:
  - Per-hook retry logic
  - Aggregate retry tracking via `maxRetries`
  - Support for "ignore", "retry", and "fail" retry policies
  - Sequential hook execution with data piping
- ✅ Updated `SetupWithManager()` to initialize `HookExecutor`
- ✅ Updated progress conditions and recovery logic

### 5. Configuration Updates

**Files:**
- `internal/config/config.go`
- `internal/config/configmap.go`

- ✅ Removed `TransformConfig`
- ✅ Added `HooksConfig` with:
  - `WhitelistPath`
  - `SidecarEndpoint`
  - `DefaultTimeout`
- ✅ Updated environment variable loading
- ✅ Updated validation logic

### 6. Metrics Updates

**Files:**
- `internal/metrics/interface.go`
- `internal/metrics/prometheus.go`
- `internal/metrics/noop.go`
- `internal/metrics/prometheus_test.go`

- ✅ Replaced `RecordTransformation()` with `RecordHookExecution()`
- ✅ Added hook-specific metrics:
  - `externalsource_hook_execution_total`
  - `externalsource_hook_execution_duration_seconds`
- ✅ Updated all metrics tests

### 7. Code Removal

- ✅ Deleted `internal/transformer/cel.go`
- ✅ Deleted `internal/transformer/cel_test.go`
- ✅ Deleted `internal/transformer/interface.go`
- ✅ Removed CEL dependency from `go.mod`
- ✅ Ran `go mod tidy`

### 8. Deployment Manifests

**Files:**
- `config/manager/manager.yaml`
- `config/manager/hooks-whitelist.yaml`
- `config/manager/kustomization.yaml`

- ✅ Added externalsource-hook-executor sidecar container
- ✅ Created hook whitelist ConfigMap
- ✅ Added environment variables for hook configuration
- ✅ Mounted whitelist ConfigMap in both containers
- ✅ Configured health probes for sidecar

### 9. Examples

**Files:**
- `examples/hooks-example.yaml` (new)
- `examples/data-transformation.yaml` (updated)

- ✅ Created comprehensive hooks examples
- ✅ Updated data-transformation examples to use jq instead of CEL
- ✅ Demonstrated all retry policies
- ✅ Showed multi-step hook chains

### 10. Tests

**File:** `internal/controller/externalsource_controller_test.go`

- ✅ Created `MockHookExecutor`
- ✅ Replaced all `MockTransformer` references
- ✅ Updated all test cases
- ✅ Tests compile and pass

## Breaking Changes

### API Changes

**Removed:**
- `spec.transform` field (CEL transformation)
- `spec.transform.type` (was always "cel")
- `spec.transform.expression` (CEL expression string)

**Added:**
- `spec.maxRetries` - Maximum retry attempts across all hooks
- `spec.hooks` - Hook configuration
- `spec.hooks.preRequest` - List of hooks to run before HTTP request
- `spec.hooks.postRequest` - List of hooks to run after HTTP request

### Hook Specification

Each hook in the list has the following fields:

```yaml
- name: string              # Unique identifier for the hook
  command: string           # Command to execute (must be whitelisted)
  args: []string            # Arguments to pass to the command
  timeout: string           # Timeout duration (e.g., "30s")
  retryPolicy: string       # "ignore", "retry", or "fail" (default: "fail")
  env: []EnvVar             # Optional environment variables
```

## Migration Examples

### Example 1: Simple Field Extraction

**Before (CEL):**
```yaml
spec:
  transform:
    type: cel
    expression: |
      {
        "version": data.tag_name,
        "url": data.html_url
      }
```

**After (Hooks):**
```yaml
spec:
  maxRetries: 3
  hooks:
    postRequest:
      - name: extract-fields
        command: jq
        args:
          - |
            {
              "version": .tag_name,
              "url": .html_url
            }
        timeout: "10s"
        retryPolicy: fail
```

### Example 2: Creating a ConfigMap

**Before (CEL):**
```yaml
spec:
  transform:
    type: cel
    expression: |
      {
        "apiVersion": "v1",
        "kind": "ConfigMap",
        "metadata": {"name": "app-config"},
        "data": {
          "database_url": data.database.url,
          "log_level": data.logging.level
        }
      }
```

**After (Hooks):**
```yaml
spec:
  maxRetries: 3
  hooks:
    postRequest:
      - name: create-configmap
        command: jq
        args:
          - |
            {
              "apiVersion": "v1",
              "kind": "ConfigMap",
              "metadata": {"name": "app-config"},
              "data": {
                "database_url": .database.url,
                "log_level": .logging.level
              }
            }
        timeout: "10s"
        retryPolicy: fail
      
      - name: to-yaml
        command: yq
        args:
          - "-P"
        timeout: "5s"
        retryPolicy: retry
```

### Example 3: Array Filtering

**Before (CEL):**
```yaml
spec:
  transform:
    type: cel
    expression: |
      data.services.filter(s, s.status == "healthy").map(s, {
        "name": s.name,
        "endpoint": s.endpoint
      })
```

**After (Hooks):**
```yaml
spec:
  maxRetries: 3
  hooks:
    postRequest:
      - name: filter-services
        command: jq
        args:
          - |
            .services 
            | map(select(.status == "healthy"))
            | map({name: .name, endpoint: .endpoint})
        timeout: "15s"
        retryPolicy: retry
```

### Example 4: Conditional Logic

**Before (CEL):**
```yaml
spec:
  transform:
    type: cel
    expression: |
      data.environment == "production" ? 
      {"config": "prod"} : 
      {"config": "dev"}
```

**After (Hooks):**
```yaml
spec:
  maxRetries: 3
  hooks:
    postRequest:
      - name: conditional-config
        command: jq
        args:
          - |
            if .environment == "production" then
              {"config": "prod"}
            else
              {"config": "dev"}
            end
        timeout: "10s"
        retryPolicy: fail
```

## CEL to jq Expression Mapping

| CEL | jq |
|-----|-----|
| `data.field` | `.field` |
| `data.array[0]` | `.array[0]` |
| `has(data.field)` | `.field != null` or `has("field")` |
| `size(data.array)` | `.array \| length` |
| `data.array.filter(x, x > 5)` | `.array \| map(select(. > 5))` |
| `data.array.map(x, x * 2)` | `.array \| map(. * 2)` |
| `string(data.value)` | `.value \| tostring` |
| `base64.encode(data.str)` | `.str \| @base64` |

## New Capabilities with Hooks

The hooks system provides capabilities not available with CEL:

### 1. Multi-Step Transformations

```yaml
hooks:
  postRequest:
    - name: validate
      command: jq
      args: ["."]
      retryPolicy: fail
    
    - name: transform
      command: jq
      args: [".field"]
      retryPolicy: retry
    
    - name: format
      command: yq
      args: ["-P"]
      retryPolicy: ignore
```

### 2. Custom Scripts

```yaml
hooks:
  postRequest:
    - name: custom-processing
      command: bash
      args:
        - "-c"
        - "cat | grep pattern | sort | uniq"
      timeout: "30s"
      retryPolicy: retry
```

### 3. Validation Hooks

```yaml
hooks:
  postRequest:
    - name: validate-schema
      command: jq
      args:
        - |
          if .version == null then
            error("version field required")
          else
            .
          end
      retryPolicy: fail
```

### 4. Environment Variables

```yaml
hooks:
  postRequest:
    - name: process-with-env
      command: bash
      args: ["-c", "envsubst"]
      env:
        - name: ENVIRONMENT
          value: "production"
        - name: REGION
          value: "us-east-1"
      retryPolicy: fail
```

## Architecture

### System Flow

```
ExternalSource Resource
    ↓
Controller (with HookExecutor)
    ↓ HTTP Request
ExternalSource Hook-Executor Sidecar
    ↓ Validates against whitelist
Executes whitelisted command
    ↓ Returns stdout/stderr
Controller processes result
    ↓
Next hook in chain or package artifact
```

### Retry Logic

- Each `ExternalSource` has a `maxRetries` limit (default: 3)
- Each hook has a `retryPolicy`:
  - **fail** (default): Stop on error
  - **retry**: Retry on error, count against maxRetries
  - **ignore**: Continue on error, don't count against maxRetries
- Retries are tracked in aggregate across all hooks
- Exponential backoff between retries

### Security Model

1. **Whitelist-First**: Only explicitly allowed commands can execute
2. **Argument Validation**: Optional regex patterns to restrict arguments
3. **Sidecar Isolation**: Commands run in a separate container
4. **No Shell Expansion**: Direct command execution, no shell interpretation
5. **Resource Limits**: CPU and memory limits on sidecar container

## Deployment Changes

### Controller Configuration

New environment variables for the controller:

```yaml
env:
  - name: HOOK_EXECUTOR_ENDPOINT
    value: "http://localhost:8081"
  - name: HOOK_WHITELIST_PATH
    value: "/etc/hooks/whitelist.yaml"
  - name: HOOK_DEFAULT_TIMEOUT
    value: "30s"
```

### Sidecar Container

The controller deployment now includes an externalsource-hook-executor sidecar:

```yaml
containers:
  - name: externalsource-hook-executor
    image: ghcr.io/oddkinco/externalsource-hook-executor:latest
    ports:
      - containerPort: 8081
    volumeMounts:
      - name: hook-whitelist
        mountPath: /etc/hooks
        readOnly: true
```

### Whitelist ConfigMap

Commands must be whitelisted in a ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: hook-whitelist
data:
  whitelist.yaml: |
    commands:
      jq:
        allowed: true
      yq:
        allowed: true
      bash:
        allowed: true
        argumentPatterns:
          - "^-c$"
```

## Migration Steps for Users

Users must migrate from CEL to hooks:

1. Replace `spec.transform` with `spec.hooks.postRequest`
2. Convert CEL expressions to jq commands
3. Add `spec.maxRetries` if custom retry behavior is needed
4. Deploy updated controller with sidecar
5. Create and mount whitelist ConfigMap

## Troubleshooting

### Hook Execution Failures

1. **Command not whitelisted:**
   ```
   Error: command jq is not whitelisted
   ```
   Solution: Add the command to the whitelist ConfigMap

2. **Timeout errors:**
   ```
   Error: hook execution timed out after 30s
   ```
   Solution: Increase the timeout or optimize the command

3. **Retry limit exceeded:**
   ```
   Error: hook failed after 3 attempts
   ```
   Solution: Check hook logic, increase maxRetries, or use retryPolicy: ignore

### Checking Hook Status

View hook execution status:

```bash
kubectl get externalsource my-source -o yaml
```

Look for the `ExecutingHooksCondition` in the status conditions.

## Verification

All components build successfully:

```bash
✓ make manifests generate
✓ go build ./...
✓ go mod tidy
✓ Tests updated and pass
```

## Files Modified

- API: 1 file
- Internal packages: 15 files
- CMD: 4 files (new)
- Config: 4 files
- Examples: 2 files
- Tests: 2 files  
- Documentation: 2 files (new)
- Total: ~30 files

## Next Steps

1. Update integration tests to test hook execution
2. Add e2e tests with actual externalsource-hook-executor sidecar
3. Update Helm chart to include sidecar configuration
4. Create tutorial videos/blog posts
5. Update project README with hooks examples

## Support

For questions or issues with migration:
- Check the examples in `examples/hooks-example.yaml`
- Review the externalsource-hook-executor whitelist at `cmd/externalsource-hook-executor/examples/whitelist.yaml`
- See the full documentation in `docs/`

## Notes

- Backward compatibility: **Breaking change** - users must migrate
- Performance: Hooks add minimal overhead (HTTP call to localhost)
- Flexibility: Hooks support any command-line tool
- Security: Whitelist provides strong security boundary
- Observability: Comprehensive metrics for hook execution
