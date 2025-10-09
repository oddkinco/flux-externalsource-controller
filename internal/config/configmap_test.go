package config

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewConfigMapLoader(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	loader := NewConfigMapLoader(k8sClient, "test-namespace", "test-config")

	assert.NotNil(t, loader)
	assert.Equal(t, "test-namespace", loader.namespace)
	assert.Equal(t, "test-config", loader.name)
}

func TestConfigMapLoader_LoadConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	tests := []struct {
		name           string
		configMap      *corev1.ConfigMap
		expectError    bool
		validateConfig func(t *testing.T, config *Config)
	}{
		{
			name: "successful load with all configuration sections",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					// Storage configuration
					"storage.backend":      "s3",
					"storage.s3.bucket":    "test-bucket",
					"storage.s3.region":    "us-west-2",
					"storage.s3.endpoint":  "https://s3.example.com",
					"storage.s3.useSSL":    "false",
					"storage.s3.pathStyle": "true",

					// HTTP configuration
					"http.timeout":             "60s",
					"http.maxIdleConns":        "200",
					"http.maxIdleConnsPerHost": "20",
					"http.maxConnsPerHost":     "200",
					"http.idleConnTimeout":     "120s",
					"http.userAgent":           "test-agent/1.0",

					// Retry configuration
					"retry.maxAttempts":  "5",
					"retry.baseDelay":    "2s",
					"retry.maxDelay":     "10m",
					"retry.jitterFactor": "0.5",

					// Transform configuration
					"transform.timeout":     "45s",
					"transform.memoryLimit": "134217728",

					// Metrics configuration
					"metrics.enabled":  "false",
					"metrics.interval": "30s",
				},
			},
			validateConfig: func(t *testing.T, config *Config) {
				// Validate storage config
				assert.Equal(t, "s3", config.Storage.Backend)
				assert.Equal(t, "test-bucket", config.Storage.S3.Bucket)
				assert.Equal(t, "us-west-2", config.Storage.S3.Region)
				assert.Equal(t, "https://s3.example.com", config.Storage.S3.Endpoint)
				assert.False(t, config.Storage.S3.UseSSL)
				assert.True(t, config.Storage.S3.PathStyle)

				// Validate HTTP config
				assert.Equal(t, 60*time.Second, config.HTTP.Timeout)
				assert.Equal(t, 200, config.HTTP.MaxIdleConns)
				assert.Equal(t, 20, config.HTTP.MaxIdleConnsPerHost)
				assert.Equal(t, 200, config.HTTP.MaxConnsPerHost)
				assert.Equal(t, 120*time.Second, config.HTTP.IdleConnTimeout)
				assert.Equal(t, "test-agent/1.0", config.HTTP.UserAgent)

				// Validate retry config
				assert.Equal(t, 5, config.Retry.MaxAttempts)
				assert.Equal(t, 2*time.Second, config.Retry.BaseDelay)
				assert.Equal(t, 10*time.Minute, config.Retry.MaxDelay)
				assert.Equal(t, 0.5, config.Retry.JitterFactor)

				// Validate transform config
				assert.Equal(t, 45*time.Second, config.Transform.Timeout)
				assert.Equal(t, int64(134217728), config.Transform.MemoryLimit)

				// Validate metrics config
				assert.False(t, config.Metrics.Enabled)
				assert.Equal(t, 30*time.Second, config.Metrics.Interval)
			},
		},
		{
			name: "partial configuration with defaults",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"storage.backend":   "s3",
					"storage.s3.bucket": "partial-bucket",
					"http.userAgent":    "partial-agent/1.0",
				},
			},
			validateConfig: func(t *testing.T, config *Config) {
				// Should have ConfigMap values
				assert.Equal(t, "s3", config.Storage.Backend)
				assert.Equal(t, "partial-bucket", config.Storage.S3.Bucket)
				assert.Equal(t, "partial-agent/1.0", config.HTTP.UserAgent)

				// Should retain default values for unspecified fields
				assert.Equal(t, "us-east-1", config.Storage.S3.Region) // default
				assert.True(t, config.Storage.S3.UseSSL)               // default
				assert.Equal(t, 30*time.Second, config.HTTP.Timeout)   // default
			},
		},
		{
			name: "invalid values fall back to defaults",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"http.timeout":          "invalid-duration",
					"http.maxIdleConns":     "not-a-number",
					"storage.s3.useSSL":     "maybe",
					"retry.jitterFactor":    "not-a-float",
					"transform.memoryLimit": "not-a-number",
				},
			},
			validateConfig: func(t *testing.T, config *Config) {
				// Should fall back to defaults for invalid values
				assert.Equal(t, 30*time.Second, config.HTTP.Timeout)
				assert.Equal(t, 100, config.HTTP.MaxIdleConns)
				assert.True(t, config.Storage.S3.UseSSL)
				assert.Equal(t, 0.25, config.Retry.JitterFactor)
				assert.Equal(t, int64(64*1024*1024), config.Transform.MemoryLimit)
			},
		},
		{
			name:        "ConfigMap not found",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client
			var k8sClient client.Client
			if tt.configMap != nil {
				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(tt.configMap).
					Build()
			} else {
				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					Build()
			}

			// Create loader
			loader := NewConfigMapLoader(k8sClient, "test-namespace", "test-config")

			// Start with default config
			config := DefaultConfig()

			// Load from ConfigMap
			err := loader.LoadConfig(context.Background(), config)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.validateConfig != nil {
				tt.validateConfig(t, config)
			}
		})
	}
}

func TestConfigMapLoader_LoadFromData(t *testing.T) {
	loader := &ConfigMapLoader{}
	config := DefaultConfig()

	data := map[string]string{
		"storage.backend":   "s3",
		"storage.s3.bucket": "test-bucket",
		"http.userAgent":    "test-agent/1.0",
		"unknown.field":     "should-be-ignored",
	}

	loader.loadFromData(data, config)

	assert.Equal(t, "s3", config.Storage.Backend)
	assert.Equal(t, "test-bucket", config.Storage.S3.Bucket)
	assert.Equal(t, "test-agent/1.0", config.HTTP.UserAgent)
	// Unknown fields should be ignored without error
}

func TestConfigMapLoader_LoadStorageConfig(t *testing.T) {
	loader := &ConfigMapLoader{}
	config := DefaultConfig()

	data := map[string]string{
		"storage.backend":      "s3",
		"storage.s3.bucket":    "test-bucket",
		"storage.s3.region":    "eu-west-1",
		"storage.s3.endpoint":  "https://custom.s3.com",
		"storage.s3.useSSL":    "false",
		"storage.s3.pathStyle": "true",
	}

	loader.loadStorageConfig(data, config)

	assert.Equal(t, "s3", config.Storage.Backend)
	assert.Equal(t, "test-bucket", config.Storage.S3.Bucket)
	assert.Equal(t, "eu-west-1", config.Storage.S3.Region)
	assert.Equal(t, "https://custom.s3.com", config.Storage.S3.Endpoint)
	assert.False(t, config.Storage.S3.UseSSL)
	assert.True(t, config.Storage.S3.PathStyle)
}

func TestConfigMapLoader_LoadHTTPConfig(t *testing.T) {
	loader := &ConfigMapLoader{}
	config := DefaultConfig()

	data := map[string]string{
		"http.timeout":             "45s",
		"http.maxIdleConns":        "150",
		"http.maxIdleConnsPerHost": "15",
		"http.maxConnsPerHost":     "150",
		"http.idleConnTimeout":     "100s",
		"http.userAgent":           "custom-agent/2.0",
	}

	loader.loadHTTPConfig(data, config)

	assert.Equal(t, 45*time.Second, config.HTTP.Timeout)
	assert.Equal(t, 150, config.HTTP.MaxIdleConns)
	assert.Equal(t, 15, config.HTTP.MaxIdleConnsPerHost)
	assert.Equal(t, 150, config.HTTP.MaxConnsPerHost)
	assert.Equal(t, 100*time.Second, config.HTTP.IdleConnTimeout)
	assert.Equal(t, "custom-agent/2.0", config.HTTP.UserAgent)
}

func TestConfigMapLoader_LoadRetryConfig(t *testing.T) {
	loader := &ConfigMapLoader{}
	config := DefaultConfig()

	data := map[string]string{
		"retry.maxAttempts":  "7",
		"retry.baseDelay":    "3s",
		"retry.maxDelay":     "15m",
		"retry.jitterFactor": "0.3",
	}

	loader.loadRetryConfig(data, config)

	assert.Equal(t, 7, config.Retry.MaxAttempts)
	assert.Equal(t, 3*time.Second, config.Retry.BaseDelay)
	assert.Equal(t, 15*time.Minute, config.Retry.MaxDelay)
	assert.Equal(t, 0.3, config.Retry.JitterFactor)
}

func TestConfigMapLoader_LoadTransformConfig(t *testing.T) {
	loader := &ConfigMapLoader{}
	config := DefaultConfig()

	data := map[string]string{
		"transform.timeout":     "60s",
		"transform.memoryLimit": "268435456", // 256MB
	}

	loader.loadTransformConfig(data, config)

	assert.Equal(t, 60*time.Second, config.Transform.Timeout)
	assert.Equal(t, int64(268435456), config.Transform.MemoryLimit)
}

func TestConfigMapLoader_LoadMetricsConfig(t *testing.T) {
	loader := &ConfigMapLoader{}
	config := DefaultConfig()

	data := map[string]string{
		"metrics.enabled":  "false",
		"metrics.interval": "45s",
	}

	loader.loadMetricsConfig(data, config)

	assert.False(t, config.Metrics.Enabled)
	assert.Equal(t, 45*time.Second, config.Metrics.Interval)
}
