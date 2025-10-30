# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/externalsource-hook-executor/ cmd/externalsource-hook-executor/
COPY internal/ internal/
COPY api/ api/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} \
    go build -a -o externalsource-hook-executor ./cmd/externalsource-hook-executor

# Runtime stage
FROM alpine:3.19

# Install common utilities for hook execution
RUN apk add --no-cache \
    bash \
    curl \
    jq \
    yq \
    ca-certificates \
    && rm -rf /var/cache/apk/*

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /workspace/externalsource-hook-executor /app/externalsource-hook-executor

# Create default whitelist directory
RUN mkdir -p /etc/hooks

# Copy example whitelist (can be overridden by volume mount)
COPY cmd/externalsource-hook-executor/examples/whitelist.yaml /etc/hooks/whitelist.yaml

# Run as non-root user
RUN addgroup -g 65532 -S hookexec && \
    adduser -u 65532 -S hookexec -G hookexec && \
    chown -R hookexec:hookexec /app /etc/hooks

USER 65532:65532

EXPOSE 8082

ENTRYPOINT ["/app/externalsource-hook-executor"]
CMD ["--port", "8082", "--whitelist", "/etc/hooks/whitelist.yaml"]

