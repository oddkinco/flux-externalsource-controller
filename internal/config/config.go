/*
Copyright (c) 2025 Odd Kin <oddkin@oddkin.co>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

// Package config provides configuration management for the ExternalSource controller.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds the configuration for the ExternalSource controller
type Config struct {
	// Storage configuration
	Storage StorageConfig `json:"storage"`

	// HTTP client configuration
	HTTP HTTPConfig `json:"http"`

	// Retry configuration
	Retry RetryConfig `json:"retry"`

	// Hooks configuration
	Hooks HooksConfig `json:"hooks"`

	// Metrics configuration
	Metrics MetricsConfig `json:"metrics"`

	// ArtifactServer configuration
	ArtifactServer ArtifactServerConfig `json:"artifactServer"`
}

// StorageConfig holds storage backend configuration
type StorageConfig struct {
	// Backend type: "s3" or "memory"
	Backend string `json:"backend"`

	// S3 configuration (used when Backend is "s3")
	S3 S3Config `json:"s3"`
}

// S3Config holds S3-compatible storage configuration
type S3Config struct {
	// Endpoint URL for S3-compatible storage
	Endpoint string `json:"endpoint"`

	// Bucket name for storing artifacts
	Bucket string `json:"bucket"`

	// Region for S3 (optional for some S3-compatible services)
	Region string `json:"region"`

	// Access key ID (can be set via environment variable)
	AccessKeyID string `json:"accessKeyId"`

	// Secret access key (can be set via environment variable)
	SecretAccessKey string `json:"secretAccessKey"`

	// Use SSL/TLS for connections
	UseSSL bool `json:"useSSL"`

	// Path style for S3 requests (required for some S3-compatible services like MinIO)
	PathStyle bool `json:"pathStyle"`
}

// HTTPConfig holds HTTP client configuration
type HTTPConfig struct {
	// Default timeout for HTTP requests
	Timeout time.Duration `json:"timeout"`

	// Maximum number of idle connections
	MaxIdleConns int `json:"maxIdleConns"`

	// Maximum number of idle connections per host
	MaxIdleConnsPerHost int `json:"maxIdleConnsPerHost"`

	// Maximum number of connections per host
	MaxConnsPerHost int `json:"maxConnsPerHost"`

	// Idle connection timeout
	IdleConnTimeout time.Duration `json:"idleConnTimeout"`

	// User agent string for HTTP requests
	UserAgent string `json:"userAgent"`
}

// RetryConfig holds retry configuration
type RetryConfig struct {
	// Maximum number of retry attempts
	MaxAttempts int `json:"maxAttempts"`

	// Base delay for exponential backoff
	BaseDelay time.Duration `json:"baseDelay"`

	// Maximum delay for exponential backoff
	MaxDelay time.Duration `json:"maxDelay"`

	// Jitter factor for randomizing retry delays (0.0 to 1.0)
	JitterFactor float64 `json:"jitterFactor"`
}

// HooksConfig holds hooks execution configuration
type HooksConfig struct {
	// WhitelistPath is the path to the whitelist configuration file
	WhitelistPath string `json:"whitelistPath"`

	// SidecarEndpoint is the HTTP endpoint for the hook executor sidecar
	SidecarEndpoint string `json:"sidecarEndpoint"`

	// DefaultTimeout is the default timeout for hook execution
	DefaultTimeout time.Duration `json:"defaultTimeout"`
}

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	// Enable detailed metrics collection
	Enabled bool `json:"enabled"`

	// Metrics collection interval
	Interval time.Duration `json:"interval"`
}

// ArtifactServerConfig holds artifact HTTP server configuration
type ArtifactServerConfig struct {
	// Enable artifact HTTP server
	Enabled bool `json:"enabled"`

	// Port for the artifact HTTP server
	Port int `json:"port"`

	// ServiceName is the Kubernetes service name for the artifact server
	ServiceName string `json:"serviceName"`

	// ServiceNamespace is the Kubernetes namespace where the service is deployed
	ServiceNamespace string `json:"serviceNamespace"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Storage: StorageConfig{
			Backend: "memory", // Default to memory for development
			S3: S3Config{
				Region:    "us-east-1",
				UseSSL:    true,
				PathStyle: false,
			},
		},
		HTTP: HTTPConfig{
			Timeout:             30 * time.Second,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			MaxConnsPerHost:     100,
			IdleConnTimeout:     90 * time.Second,
			UserAgent:           "externalsource-controller/1.0",
		},
		Retry: RetryConfig{
			MaxAttempts:  10,
			BaseDelay:    1 * time.Second,
			MaxDelay:     5 * time.Minute,
			JitterFactor: 0.25,
		},
		Hooks: HooksConfig{
			WhitelistPath:   "/etc/hooks/whitelist.yaml",
			SidecarEndpoint: "http://localhost:8082",
			DefaultTimeout:  30 * time.Second,
		},
		Metrics: MetricsConfig{
			Enabled:  true,
			Interval: 15 * time.Second,
		},
		ArtifactServer: ArtifactServerConfig{
			Enabled:          true,
			Port:             8080,
			ServiceName:      "flux-externalsource-controller-artifacts",
			ServiceNamespace: "flux-system",
		},
	}
}

// LoadFromEnvironment loads configuration from environment variables
func (c *Config) LoadFromEnvironment() {
	c.loadStorageFromEnv()
	c.loadHTTPFromEnv()
	c.loadRetryFromEnv()
	c.loadHooksFromEnv()
	c.loadMetricsFromEnv()
	c.loadArtifactServerFromEnv()
}

// loadStorageFromEnv loads storage configuration from environment variables
func (c *Config) loadStorageFromEnv() {
	if backend := os.Getenv("STORAGE_BACKEND"); backend != "" {
		c.Storage.Backend = backend
	}

	// S3 configuration
	if endpoint := os.Getenv("S3_ENDPOINT"); endpoint != "" {
		c.Storage.S3.Endpoint = endpoint
	}
	if bucket := os.Getenv("S3_BUCKET"); bucket != "" {
		c.Storage.S3.Bucket = bucket
	}
	if region := os.Getenv("S3_REGION"); region != "" {
		c.Storage.S3.Region = region
	}
	if accessKeyID := os.Getenv("S3_ACCESS_KEY_ID"); accessKeyID != "" {
		c.Storage.S3.AccessKeyID = accessKeyID
	}
	if secretAccessKey := os.Getenv("S3_SECRET_ACCESS_KEY"); secretAccessKey != "" {
		c.Storage.S3.SecretAccessKey = secretAccessKey
	}
	if useSSLStr := os.Getenv("S3_USE_SSL"); useSSLStr != "" {
		if useSSL, err := strconv.ParseBool(useSSLStr); err == nil {
			c.Storage.S3.UseSSL = useSSL
		}
	}
	if pathStyleStr := os.Getenv("S3_PATH_STYLE"); pathStyleStr != "" {
		if pathStyle, err := strconv.ParseBool(pathStyleStr); err == nil {
			c.Storage.S3.PathStyle = pathStyle
		}
	}
}

// loadHTTPFromEnv loads HTTP configuration from environment variables
func (c *Config) loadHTTPFromEnv() {
	if timeoutStr := os.Getenv("HTTP_TIMEOUT"); timeoutStr != "" {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			c.HTTP.Timeout = timeout
		}
	}
	if maxIdleConnsStr := os.Getenv("HTTP_MAX_IDLE_CONNS"); maxIdleConnsStr != "" {
		if maxIdleConns, err := strconv.Atoi(maxIdleConnsStr); err == nil {
			c.HTTP.MaxIdleConns = maxIdleConns
		}
	}
	if maxIdleConnsPerHostStr := os.Getenv("HTTP_MAX_IDLE_CONNS_PER_HOST"); maxIdleConnsPerHostStr != "" {
		if maxIdleConnsPerHost, err := strconv.Atoi(maxIdleConnsPerHostStr); err == nil {
			c.HTTP.MaxIdleConnsPerHost = maxIdleConnsPerHost
		}
	}
	if maxConnsPerHostStr := os.Getenv("HTTP_MAX_CONNS_PER_HOST"); maxConnsPerHostStr != "" {
		if maxConnsPerHost, err := strconv.Atoi(maxConnsPerHostStr); err == nil {
			c.HTTP.MaxConnsPerHost = maxConnsPerHost
		}
	}
	if idleConnTimeoutStr := os.Getenv("HTTP_IDLE_CONN_TIMEOUT"); idleConnTimeoutStr != "" {
		if idleConnTimeout, err := time.ParseDuration(idleConnTimeoutStr); err == nil {
			c.HTTP.IdleConnTimeout = idleConnTimeout
		}
	}
	if userAgent := os.Getenv("HTTP_USER_AGENT"); userAgent != "" {
		c.HTTP.UserAgent = userAgent
	}
}

// loadRetryFromEnv loads retry configuration from environment variables
func (c *Config) loadRetryFromEnv() {
	if maxAttemptsStr := os.Getenv("RETRY_MAX_ATTEMPTS"); maxAttemptsStr != "" {
		if maxAttempts, err := strconv.Atoi(maxAttemptsStr); err == nil {
			c.Retry.MaxAttempts = maxAttempts
		}
	}
	if baseDelayStr := os.Getenv("RETRY_BASE_DELAY"); baseDelayStr != "" {
		if baseDelay, err := time.ParseDuration(baseDelayStr); err == nil {
			c.Retry.BaseDelay = baseDelay
		}
	}
	if maxDelayStr := os.Getenv("RETRY_MAX_DELAY"); maxDelayStr != "" {
		if maxDelay, err := time.ParseDuration(maxDelayStr); err == nil {
			c.Retry.MaxDelay = maxDelay
		}
	}
	if jitterFactorStr := os.Getenv("RETRY_JITTER_FACTOR"); jitterFactorStr != "" {
		if jitterFactor, err := strconv.ParseFloat(jitterFactorStr, 64); err == nil {
			c.Retry.JitterFactor = jitterFactor
		}
	}
}

// loadHooksFromEnv loads hooks configuration from environment variables
func (c *Config) loadHooksFromEnv() {
	if whitelistPath := os.Getenv("HOOK_WHITELIST_PATH"); whitelistPath != "" {
		c.Hooks.WhitelistPath = whitelistPath
	}
	if sidecarEndpoint := os.Getenv("HOOK_EXECUTOR_ENDPOINT"); sidecarEndpoint != "" {
		c.Hooks.SidecarEndpoint = sidecarEndpoint
	}
	if timeoutStr := os.Getenv("HOOK_DEFAULT_TIMEOUT"); timeoutStr != "" {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			c.Hooks.DefaultTimeout = timeout
		}
	}
}

// loadMetricsFromEnv loads metrics configuration from environment variables
func (c *Config) loadMetricsFromEnv() {
	if enabledStr := os.Getenv("METRICS_ENABLED"); enabledStr != "" {
		if enabled, err := strconv.ParseBool(enabledStr); err == nil {
			c.Metrics.Enabled = enabled
		}
	}
	if intervalStr := os.Getenv("METRICS_INTERVAL"); intervalStr != "" {
		if interval, err := time.ParseDuration(intervalStr); err == nil {
			c.Metrics.Interval = interval
		}
	}
}

// loadArtifactServerFromEnv loads artifact server configuration from environment variables
func (c *Config) loadArtifactServerFromEnv() {
	if enabledStr := os.Getenv("ARTIFACT_SERVER_ENABLED"); enabledStr != "" {
		if enabled, err := strconv.ParseBool(enabledStr); err == nil {
			c.ArtifactServer.Enabled = enabled
		}
	}
	if portStr := os.Getenv("ARTIFACT_SERVER_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			c.ArtifactServer.Port = port
		}
	}
	if serviceName := os.Getenv("SERVICE_NAME"); serviceName != "" {
		c.ArtifactServer.ServiceName = serviceName
	}
	if serviceNamespace := os.Getenv("POD_NAMESPACE"); serviceNamespace != "" {
		c.ArtifactServer.ServiceNamespace = serviceNamespace
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate storage configuration
	if c.Storage.Backend != "s3" && c.Storage.Backend != "memory" {
		return fmt.Errorf("invalid storage backend: %s (must be 's3' or 'memory')", c.Storage.Backend)
	}

	if c.Storage.Backend == "s3" {
		if c.Storage.S3.Endpoint == "" {
			return fmt.Errorf("S3 endpoint is required when using S3 storage backend")
		}
		if c.Storage.S3.Bucket == "" {
			return fmt.Errorf("S3 bucket is required when using S3 storage backend")
		}
		if c.Storage.S3.AccessKeyID == "" {
			return fmt.Errorf("S3 access key ID is required when using S3 storage backend")
		}
		if c.Storage.S3.SecretAccessKey == "" {
			return fmt.Errorf("S3 secret access key is required when using S3 storage backend")
		}
	}

	// Validate HTTP configuration
	if c.HTTP.Timeout <= 0 {
		return fmt.Errorf("HTTP timeout must be positive")
	}
	if c.HTTP.MaxIdleConns < 0 {
		return fmt.Errorf("HTTP max idle connections must be non-negative")
	}
	if c.HTTP.MaxIdleConnsPerHost < 0 {
		return fmt.Errorf("HTTP max idle connections per host must be non-negative")
	}
	if c.HTTP.MaxConnsPerHost < 0 {
		return fmt.Errorf("HTTP max connections per host must be non-negative")
	}
	if c.HTTP.IdleConnTimeout <= 0 {
		return fmt.Errorf("HTTP idle connection timeout must be positive")
	}

	// Validate retry configuration
	if c.Retry.MaxAttempts < 0 {
		return fmt.Errorf("retry max attempts must be non-negative")
	}
	if c.Retry.BaseDelay <= 0 {
		return fmt.Errorf("retry base delay must be positive")
	}
	if c.Retry.MaxDelay <= 0 {
		return fmt.Errorf("retry max delay must be positive")
	}
	if c.Retry.BaseDelay > c.Retry.MaxDelay {
		return fmt.Errorf("retry base delay must be less than or equal to max delay")
	}
	if c.Retry.JitterFactor < 0 || c.Retry.JitterFactor > 1 {
		return fmt.Errorf("retry jitter factor must be between 0 and 1")
	}

	// Validate hooks configuration
	if c.Hooks.WhitelistPath == "" {
		return fmt.Errorf("hooks whitelist path must be specified")
	}
	if c.Hooks.SidecarEndpoint == "" {
		return fmt.Errorf("hooks sidecar endpoint must be specified")
	}
	if c.Hooks.DefaultTimeout <= 0 {
		return fmt.Errorf("hooks default timeout must be positive")
	}

	// Validate metrics configuration
	if c.Metrics.Interval <= 0 {
		return fmt.Errorf("metrics interval must be positive")
	}

	// Validate artifact server configuration
	if c.ArtifactServer.Port < 1 || c.ArtifactServer.Port > 65535 {
		return fmt.Errorf("artifact server port must be between 1 and 65535")
	}
	if c.ArtifactServer.ServiceName == "" {
		return fmt.Errorf("artifact server service name must be specified")
	}
	if c.ArtifactServer.ServiceNamespace == "" {
		return fmt.Errorf("artifact server service namespace must be specified")
	}

	return nil
}
