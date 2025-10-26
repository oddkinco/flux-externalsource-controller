# Migration Guide: CEL to Hooks System

This guide helps you migrate from the CEL-based transformation system to the new flexible hooks system.

## Overview of Changes

The ExternalSource controller has been updated to replace CEL (Common Expression Language) transformations with a more flexible hook system that allows:

- Pre-request and post-request command execution
- Multiple hooks chained in sequence
- Configurable retry policies per hook
- Whitelisted command execution for security
- Support for any command-line tool (jq, yq, bash, etc.)

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

The controller deployment now includes a hook-executor sidecar:

```yaml
containers:
  - name: hook-executor
    image: ghcr.io/oddkinco/hook-executor:latest
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

## Support

For questions or issues with migration:
- Check the examples in `examples/hooks-example.yaml`
- Review the hook-executor whitelist at `cmd/hook-executor/examples/whitelist.yaml`
- See the full documentation in `docs/`

