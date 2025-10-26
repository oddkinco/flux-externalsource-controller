# Hook Executor

The hook-executor is a sidecar binary that executes whitelisted commands for the ExternalSource controller. It provides a secure HTTP API for running commands with stdin/stdout streaming and environment variable support.

## Features

- **Whitelist-based security**: Only commands explicitly allowed in the whitelist can be executed
- **Argument validation**: Optional regex patterns to restrict command arguments
- **Timeout enforcement**: Each command execution has a configurable timeout
- **Environment variable support**: Commands can receive custom environment variables
- **Stdin/stdout streaming**: Binary-safe input/output via base64 encoding
- **Health checks**: Built-in health endpoint for container orchestration

## Usage

### Running the Server

```bash
hook-executor --port 8081 --whitelist /etc/hooks/whitelist.yaml
```

### Command Line Options

- `--port`: Port to listen on (default: 8081)
- `--whitelist`: Path to whitelist configuration file (default: /etc/hooks/whitelist.yaml)

### API Endpoints

#### POST /execute

Executes a whitelisted command with the provided parameters.

**Request Body:**

```json
{
  "command": "jq",
  "args": [".field", "-r"],
  "timeout": "30s",
  "env": {
    "FOO": "bar"
  },
  "stdin": "eyJmaWVsZCI6ICJ2YWx1ZSJ9"  // base64 encoded input
}
```

**Response:**

```json
{
  "stdout": "dmFsdWU=",  // base64 encoded output
  "stderr": "",          // base64 encoded stderr
  "exitCode": 0
}
```

**Status Codes:**

- `200 OK`: Command executed successfully (check exitCode in response)
- `400 Bad Request`: Invalid request format
- `403 Forbidden`: Command not whitelisted
- `405 Method Not Allowed`: Non-POST request

#### GET /health

Health check endpoint for monitoring.

**Response:**

```json
{
  "status": "healthy"
}
```

## Whitelist Configuration

The whitelist is defined in a YAML file that specifies allowed commands and optional argument restrictions.

### Format

```yaml
commands:
  <command-name>:
    allowed: <bool>
    argumentPatterns:
      - "<regex-pattern>"
      - "<regex-pattern>"
```

### Example

```yaml
commands:
  # Allow jq with restricted arguments
  jq:
    allowed: true
    argumentPatterns:
      - "^\\..*"        # Field selectors
      - "^-[a-zA-Z]$"   # Single-letter flags

  # Explicitly disallow dangerous commands
  rm:
    allowed: false
```

See [examples/whitelist.yaml](examples/whitelist.yaml) for a complete example.

### Argument Patterns

- If `argumentPatterns` is omitted or empty, all arguments are allowed
- Each argument must match at least one pattern to be allowed
- Patterns are regular expressions matched against the entire argument

## Security Considerations

1. **Whitelist-first approach**: Only explicitly allowed commands can be executed
2. **Argument validation**: Use regex patterns to restrict dangerous argument combinations
3. **No shell expansion**: Commands are executed directly without shell interpretation
4. **Timeout enforcement**: All commands have a maximum execution time
5. **Resource limits**: Run in a container with appropriate CPU/memory limits

## Docker Image

The hook-executor is available as a Docker image:

```
ghcr.io/oddkinco/hook-executor:latest
```

### Building

```bash
docker build -t hook-executor -f cmd/hook-executor/Dockerfile .
```

### Running with Docker

```bash
docker run -p 8081:8081 \
  -v $(pwd)/whitelist.yaml:/etc/hooks/whitelist.yaml:ro \
  ghcr.io/oddkinco/hook-executor:latest
```

## Integration with ExternalSource Controller

The hook-executor runs as a sidecar container alongside the ExternalSource controller. The controller communicates with it via HTTP on localhost.

Example Kubernetes deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: externalsource-controller
spec:
  template:
    spec:
      containers:
      - name: manager
        image: ghcr.io/oddkinco/flux-externalsource-controller:latest
        env:
        - name: HOOK_EXECUTOR_ENDPOINT
          value: "http://localhost:8081"
      - name: hook-executor
        image: ghcr.io/oddkinco/hook-executor:latest
        ports:
        - containerPort: 8081
        volumeMounts:
        - name: hook-whitelist
          mountPath: /etc/hooks
          readOnly: true
      volumes:
      - name: hook-whitelist
        configMap:
          name: hook-whitelist
```

## Development

### Prerequisites

- Go 1.22 or later

### Building from Source

```bash
go build -o hook-executor ./cmd/hook-executor
```

### Running Tests

```bash
go test ./cmd/hook-executor/...
```

## License

Copyright (c) 2025 Odd Kin. Licensed under the MIT License.

