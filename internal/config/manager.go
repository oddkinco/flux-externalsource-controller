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

package config

import (
	"context"
	"fmt"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Manager manages configuration loading from multiple sources
type Manager struct {
	client             client.Client
	configMapName      string
	configMapNamespace string
}

// NewManager creates a new configuration manager
func NewManager(k8sClient client.Client) *Manager {
	return &Manager{
		client:             k8sClient,
		configMapName:      getEnvOrDefault("CONFIG_MAP_NAME", "externalsource-controller-config"),
		configMapNamespace: getEnvOrDefault("CONFIG_MAP_NAMESPACE", "externalsource-controller-system"),
	}
}

// LoadConfig loads configuration from all available sources
// Priority order: ConfigMap -> Environment Variables -> Defaults
func (m *Manager) LoadConfig(ctx context.Context) (*Config, error) {
	logger := log.FromContext(ctx)

	// Start with default configuration
	config := DefaultConfig()

	// Load from ConfigMap if available (optional)
	if m.client != nil {
		configMapLoader := NewConfigMapLoader(m.client, m.configMapNamespace, m.configMapName)
		if err := configMapLoader.LoadConfig(ctx, config); err != nil {
			logger.Info("ConfigMap not found or failed to load, using environment variables and defaults",
				"configmap", fmt.Sprintf("%s/%s", m.configMapNamespace, m.configMapName),
				"error", err.Error())
		} else {
			logger.Info("Configuration loaded from ConfigMap",
				"configmap", fmt.Sprintf("%s/%s", m.configMapNamespace, m.configMapName))
		}
	}

	// Override with environment variables (highest priority)
	config.LoadFromEnvironment()
	logger.Info("Configuration loaded from environment variables")

	// Validate the final configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	logger.Info("Configuration loaded successfully",
		"storage_backend", config.Storage.Backend,
		"http_timeout", config.HTTP.Timeout,
		"retry_max_attempts", config.Retry.MaxAttempts,
		"metrics_enabled", config.Metrics.Enabled)

	return config, nil
}

// SetConfigMapSource sets the ConfigMap source for configuration
func (m *Manager) SetConfigMapSource(namespace, name string) {
	m.configMapNamespace = namespace
	m.configMapName = name
}

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
