package config

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewManager(t *testing.T) {
	// Save original environment
	originalConfigMapName := os.Getenv("CONFIG_MAP_NAME")
	originalConfigMapNamespace := os.Getenv("CONFIG_MAP_NAMESPACE")
	defer func() {
		if originalConfigMapName != "" {
			_ = os.Setenv("CONFIG_MAP_NAME", originalConfigMapName)
		} else {
			_ = os.Unsetenv("CONFIG_MAP_NAME")
		}
		if originalConfigMapNamespace != "" {
			_ = os.Setenv("CONFIG_MAP_NAMESPACE", originalConfigMapNamespace)
		} else {
			_ = os.Unsetenv("CONFIG_MAP_NAMESPACE")
		}
	}()

	tests := []struct {
		name                       string
		configMapName              string
		configMapNamespace         string
		expectedConfigMapName      string
		expectedConfigMapNamespace string
	}{
		{
			name:                       "default values",
			expectedConfigMapName:      "externalsource-controller-config",
			expectedConfigMapNamespace: "externalsource-controller-system",
		},
		{
			name:                       "custom values from environment",
			configMapName:              "custom-config",
			configMapNamespace:         "custom-namespace",
			expectedConfigMapName:      "custom-config",
			expectedConfigMapNamespace: "custom-namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables if provided
			if tt.configMapName != "" {
				_ = os.Setenv("CONFIG_MAP_NAME", tt.configMapName)
			} else {
				_ = os.Unsetenv("CONFIG_MAP_NAME")
			}
			if tt.configMapNamespace != "" {
				_ = os.Setenv("CONFIG_MAP_NAMESPACE", tt.configMapNamespace)
			} else {
				_ = os.Unsetenv("CONFIG_MAP_NAMESPACE")
			}

			manager := NewManager(nil)

			assert.Equal(t, tt.expectedConfigMapName, manager.configMapName)
			assert.Equal(t, tt.expectedConfigMapNamespace, manager.configMapNamespace)
		})
	}
}

func TestManager_LoadConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	tests := []struct {
		name           string
		configMap      *corev1.ConfigMap
		envVars        map[string]string
		expectError    bool
		validateConfig func(t *testing.T, config *Config)
	}{
		{
			name: "load from defaults only",
			validateConfig: func(t *testing.T, config *Config) {
				// Should have default values
				assert.Equal(t, "memory", config.Storage.Backend)
				assert.Equal(t, "externalsource-controller/1.0", config.HTTP.UserAgent)
			},
		},
		{
			name: "load from environment variables",
			envVars: map[string]string{
				"STORAGE_BACKEND": "memory",
				"HTTP_USER_AGENT": "test-agent/1.0",
			},
			validateConfig: func(t *testing.T, config *Config) {
				assert.Equal(t, "memory", config.Storage.Backend)
				assert.Equal(t, "test-agent/1.0", config.HTTP.UserAgent)
			},
		},
		{
			name: "load from ConfigMap",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "externalsource-controller-config",
					Namespace: "externalsource-controller-system",
				},
				Data: map[string]string{
					"storage.backend": "memory",
					"http.userAgent":  "configmap-agent/1.0",
				},
			},
			validateConfig: func(t *testing.T, config *Config) {
				assert.Equal(t, "memory", config.Storage.Backend)
				assert.Equal(t, "configmap-agent/1.0", config.HTTP.UserAgent)
			},
		},
		{
			name: "environment variables override ConfigMap",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "externalsource-controller-config",
					Namespace: "externalsource-controller-system",
				},
				Data: map[string]string{
					"storage.backend": "memory",
					"http.userAgent":  "configmap-agent/1.0",
				},
			},
			envVars: map[string]string{
				"STORAGE_BACKEND": "memory", // This should override ConfigMap
				"HTTP_USER_AGENT": "env-agent/1.0",
			},
			validateConfig: func(t *testing.T, config *Config) {
				// Environment variables should take precedence
				assert.Equal(t, "memory", config.Storage.Backend)
				assert.Equal(t, "env-agent/1.0", config.HTTP.UserAgent)
			},
		},
		{
			name: "invalid configuration fails validation",
			envVars: map[string]string{
				"STORAGE_BACKEND": "invalid-backend",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore environment
			originalEnv := make(map[string]string)
			for key := range tt.envVars {
				if val, exists := os.LookupEnv(key); exists {
					originalEnv[key] = val
				}
				_ = os.Unsetenv(key)
			}
			defer func() {
				for key := range tt.envVars {
					_ = os.Unsetenv(key)
					if val, exists := originalEnv[key]; exists {
						_ = os.Setenv(key, val)
					}
				}
			}()

			// Set test environment variables
			for key, value := range tt.envVars {
				_ = os.Setenv(key, value)
			}

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

			// Create manager and load config
			manager := NewManager(k8sClient)
			config, err := manager.LoadConfig(context.Background())

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, config)

			if tt.validateConfig != nil {
				tt.validateConfig(t, config)
			}
		})
	}
}

func TestManager_LoadConfigWithoutClient(t *testing.T) {
	// Test loading config when no Kubernetes client is available
	manager := NewManager(nil)

	// Set some environment variables
	_ = os.Setenv("STORAGE_BACKEND", "memory")
	_ = os.Setenv("HTTP_USER_AGENT", "test-agent/1.0")
	defer func() {
		_ = os.Unsetenv("STORAGE_BACKEND")
		_ = os.Unsetenv("HTTP_USER_AGENT")
	}()

	config, err := manager.LoadConfig(context.Background())

	require.NoError(t, err)
	require.NotNil(t, config)

	// Should load from environment and defaults
	assert.Equal(t, "memory", config.Storage.Backend)
	assert.Equal(t, "test-agent/1.0", config.HTTP.UserAgent)
}

func TestManager_SetConfigMapSource(t *testing.T) {
	manager := NewManager(nil)

	// Initial values should be defaults
	assert.Equal(t, "externalsource-controller-config", manager.configMapName)
	assert.Equal(t, "externalsource-controller-system", manager.configMapNamespace)

	// Set custom values
	manager.SetConfigMapSource("custom-namespace", "custom-config")

	assert.Equal(t, "custom-config", manager.configMapName)
	assert.Equal(t, "custom-namespace", manager.configMapNamespace)
}

func TestGetEnvOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		envVar       string
		envValue     string
		defaultValue string
		expected     string
	}{
		{
			name:         "environment variable exists",
			envVar:       "TEST_VAR",
			envValue:     "test-value",
			defaultValue: "default-value",
			expected:     "test-value",
		},
		{
			name:         "environment variable does not exist",
			envVar:       "NONEXISTENT_VAR",
			defaultValue: "default-value",
			expected:     "default-value",
		},
		{
			name:         "environment variable is empty",
			envVar:       "EMPTY_VAR",
			envValue:     "",
			defaultValue: "default-value",
			expected:     "default-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up environment
			_ = os.Unsetenv(tt.envVar)

			// Set environment variable if provided
			if tt.envValue != "" {
				_ = os.Setenv(tt.envVar, tt.envValue)
				defer func() { _ = os.Unsetenv(tt.envVar) }()
			}

			result := getEnvOrDefault(tt.envVar, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}
