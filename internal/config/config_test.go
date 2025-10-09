package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Test storage defaults
	assert.Equal(t, "memory", config.Storage.Backend)
	assert.Equal(t, "us-east-1", config.Storage.S3.Region)
	assert.True(t, config.Storage.S3.UseSSL)
	assert.False(t, config.Storage.S3.PathStyle)

	// Test HTTP defaults
	assert.Equal(t, 30*time.Second, config.HTTP.Timeout)
	assert.Equal(t, 100, config.HTTP.MaxIdleConns)
	assert.Equal(t, 10, config.HTTP.MaxIdleConnsPerHost)
	assert.Equal(t, 100, config.HTTP.MaxConnsPerHost)
	assert.Equal(t, 90*time.Second, config.HTTP.IdleConnTimeout)
	assert.Equal(t, "externalsource-controller/1.0", config.HTTP.UserAgent)

	// Test retry defaults
	assert.Equal(t, 10, config.Retry.MaxAttempts)
	assert.Equal(t, 1*time.Second, config.Retry.BaseDelay)
	assert.Equal(t, 5*time.Minute, config.Retry.MaxDelay)
	assert.Equal(t, 0.25, config.Retry.JitterFactor)

	// Test transform defaults
	assert.Equal(t, 30*time.Second, config.Transform.Timeout)
	assert.Equal(t, int64(64*1024*1024), config.Transform.MemoryLimit)

	// Test metrics defaults
	assert.True(t, config.Metrics.Enabled)
	assert.Equal(t, 15*time.Second, config.Metrics.Interval)
}

func TestLoadFromEnvironment(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	envVars := []string{
		"STORAGE_BACKEND", "S3_BUCKET", "S3_REGION", "S3_ENDPOINT", "S3_USE_SSL", "S3_PATH_STYLE",
		"HTTP_TIMEOUT", "HTTP_MAX_IDLE_CONNS", "HTTP_MAX_IDLE_CONNS_PER_HOST", "HTTP_MAX_CONNS_PER_HOST",
		"HTTP_IDLE_CONN_TIMEOUT", "HTTP_USER_AGENT",
		"RETRY_MAX_ATTEMPTS", "RETRY_BASE_DELAY", "RETRY_MAX_DELAY", "RETRY_JITTER_FACTOR",
		"TRANSFORM_TIMEOUT", "TRANSFORM_MEMORY_LIMIT",
		"METRICS_ENABLED", "METRICS_INTERVAL",
	}

	for _, env := range envVars {
		if val, exists := os.LookupEnv(env); exists {
			originalEnv[env] = val
		}
		_ = os.Unsetenv(env)
	}

	// Restore environment after test
	defer func() {
		for _, env := range envVars {
			_ = os.Unsetenv(env)
			if val, exists := originalEnv[env]; exists {
				_ = os.Setenv(env, val)
			}
		}
	}()

	tests := []struct {
		name     string
		envVars  map[string]string
		validate func(t *testing.T, config *Config)
	}{
		{
			name: "storage configuration",
			envVars: map[string]string{
				"STORAGE_BACKEND": "s3",
				"S3_BUCKET":       "test-bucket",
				"S3_REGION":       "us-west-2",
				"S3_ENDPOINT":     "https://s3.example.com",
				"S3_USE_SSL":      "false",
				"S3_PATH_STYLE":   "true",
			},
			validate: func(t *testing.T, config *Config) {
				assert.Equal(t, "s3", config.Storage.Backend)
				assert.Equal(t, "test-bucket", config.Storage.S3.Bucket)
				assert.Equal(t, "us-west-2", config.Storage.S3.Region)
				assert.Equal(t, "https://s3.example.com", config.Storage.S3.Endpoint)
				assert.False(t, config.Storage.S3.UseSSL)
				assert.True(t, config.Storage.S3.PathStyle)
			},
		},
		{
			name: "http configuration",
			envVars: map[string]string{
				"HTTP_TIMEOUT":                 "60s",
				"HTTP_MAX_IDLE_CONNS":          "200",
				"HTTP_MAX_IDLE_CONNS_PER_HOST": "20",
				"HTTP_MAX_CONNS_PER_HOST":      "200",
				"HTTP_IDLE_CONN_TIMEOUT":       "120s",
				"HTTP_USER_AGENT":              "test-agent/2.0",
			},
			validate: func(t *testing.T, config *Config) {
				assert.Equal(t, 60*time.Second, config.HTTP.Timeout)
				assert.Equal(t, 200, config.HTTP.MaxIdleConns)
				assert.Equal(t, 20, config.HTTP.MaxIdleConnsPerHost)
				assert.Equal(t, 200, config.HTTP.MaxConnsPerHost)
				assert.Equal(t, 120*time.Second, config.HTTP.IdleConnTimeout)
				assert.Equal(t, "test-agent/2.0", config.HTTP.UserAgent)
			},
		},
		{
			name: "retry configuration",
			envVars: map[string]string{
				"RETRY_MAX_ATTEMPTS":  "5",
				"RETRY_BASE_DELAY":    "2s",
				"RETRY_MAX_DELAY":     "10m",
				"RETRY_JITTER_FACTOR": "0.5",
			},
			validate: func(t *testing.T, config *Config) {
				assert.Equal(t, 5, config.Retry.MaxAttempts)
				assert.Equal(t, 2*time.Second, config.Retry.BaseDelay)
				assert.Equal(t, 10*time.Minute, config.Retry.MaxDelay)
				assert.Equal(t, 0.5, config.Retry.JitterFactor)
			},
		},
		{
			name: "transform configuration",
			envVars: map[string]string{
				"TRANSFORM_TIMEOUT":      "45s",
				"TRANSFORM_MEMORY_LIMIT": "134217728", // 128MB
			},
			validate: func(t *testing.T, config *Config) {
				assert.Equal(t, 45*time.Second, config.Transform.Timeout)
				assert.Equal(t, int64(134217728), config.Transform.MemoryLimit)
			},
		},
		{
			name: "metrics configuration",
			envVars: map[string]string{
				"METRICS_ENABLED":  "false",
				"METRICS_INTERVAL": "30s",
			},
			validate: func(t *testing.T, config *Config) {
				assert.False(t, config.Metrics.Enabled)
				assert.Equal(t, 30*time.Second, config.Metrics.Interval)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for key, value := range tt.envVars {
				_ = os.Setenv(key, value)
			}

			// Create config and load from environment
			config := DefaultConfig()
			config.LoadFromEnvironment()

			// Validate
			tt.validate(t, config)

			// Clean up environment variables
			for key := range tt.envVars {
				_ = os.Unsetenv(key)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid default config",
			config:      DefaultConfig(),
			expectError: false,
		},
		{
			name: "invalid storage backend",
			config: &Config{
				Storage: StorageConfig{
					Backend: "invalid",
				},
			},
			expectError: true,
			errorMsg:    "invalid storage backend",
		},
		{
			name: "missing S3 endpoint when using S3",
			config: &Config{
				Storage: StorageConfig{
					Backend: "s3",
					S3: S3Config{
						Bucket: "test-bucket",
					},
				},
			},
			expectError: true,
			errorMsg:    "S3 endpoint is required",
		},
		{
			name: "invalid HTTP timeout",
			config: &Config{
				Storage: StorageConfig{Backend: "memory"},
				HTTP: HTTPConfig{
					Timeout: -1 * time.Second,
				},
			},
			expectError: true,
			errorMsg:    "HTTP timeout must be positive",
		},
		{
			name: "invalid retry max attempts",
			config: &Config{
				Storage: StorageConfig{Backend: "memory"},
				HTTP:    HTTPConfig{Timeout: 30 * time.Second, IdleConnTimeout: 90 * time.Second},
				Retry: RetryConfig{
					MaxAttempts: -1,
				},
			},
			expectError: true,
			errorMsg:    "retry max attempts must be non-negative",
		},
		{
			name: "invalid transform timeout",
			config: &Config{
				Storage:   StorageConfig{Backend: "memory"},
				HTTP:      HTTPConfig{Timeout: 30 * time.Second, IdleConnTimeout: 90 * time.Second},
				Retry:     RetryConfig{MaxAttempts: 3, BaseDelay: 1 * time.Second, MaxDelay: 5 * time.Minute},
				Transform: TransformConfig{Timeout: 0},
			},
			expectError: true,
			errorMsg:    "transform timeout must be positive",
		},
		{
			name: "invalid metrics interval",
			config: &Config{
				Storage:   StorageConfig{Backend: "memory"},
				HTTP:      HTTPConfig{Timeout: 30 * time.Second, IdleConnTimeout: 90 * time.Second},
				Retry:     RetryConfig{MaxAttempts: 3, BaseDelay: 1 * time.Second, MaxDelay: 5 * time.Minute},
				Transform: TransformConfig{Timeout: 30 * time.Second, MemoryLimit: 64 * 1024 * 1024},
				Metrics: MetricsConfig{
					Enabled:  true,
					Interval: -1 * time.Second,
				},
			},
			expectError: true,
			errorMsg:    "metrics interval must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoadFromEnvironmentWithInvalidValues(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	envVars := []string{
		"HTTP_TIMEOUT", "HTTP_MAX_IDLE_CONNS", "RETRY_MAX_ATTEMPTS",
		"RETRY_JITTER_FACTOR", "TRANSFORM_MEMORY_LIMIT", "METRICS_ENABLED",
	}

	for _, env := range envVars {
		if val, exists := os.LookupEnv(env); exists {
			originalEnv[env] = val
		}
		_ = os.Unsetenv(env)
	}

	// Restore environment after test
	defer func() {
		for _, env := range envVars {
			_ = os.Unsetenv(env)
			if val, exists := originalEnv[env]; exists {
				_ = os.Setenv(env, val)
			}
		}
	}()

	tests := []struct {
		name   string
		envVar string
		value  string
		check  func(t *testing.T, config *Config)
	}{
		{
			name:   "invalid timeout falls back to default",
			envVar: "HTTP_TIMEOUT",
			value:  "invalid",
			check: func(t *testing.T, config *Config) {
				// Should fall back to default
				assert.Equal(t, 30*time.Second, config.HTTP.Timeout)
			},
		},
		{
			name:   "invalid int falls back to default",
			envVar: "HTTP_MAX_IDLE_CONNS",
			value:  "not-a-number",
			check: func(t *testing.T, config *Config) {
				// Should fall back to default
				assert.Equal(t, 100, config.HTTP.MaxIdleConns)
			},
		},
		{
			name:   "invalid bool falls back to default",
			envVar: "METRICS_ENABLED",
			value:  "maybe",
			check: func(t *testing.T, config *Config) {
				// Should fall back to default
				assert.True(t, config.Metrics.Enabled)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Setenv(tt.envVar, tt.value)

			config := DefaultConfig()
			config.LoadFromEnvironment()

			tt.check(t, config)

			_ = os.Unsetenv(tt.envVar)
		})
	}
}
