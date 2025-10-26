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
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConfigMapLoader loads configuration from a Kubernetes ConfigMap
//
//nolint:revive // Clear naming is more important than avoiding "stuttering"
type ConfigMapLoader struct {
	client    client.Client
	namespace string
	name      string
}

// NewConfigMapLoader creates a new ConfigMap loader
func NewConfigMapLoader(k8sClient client.Client, namespace, name string) *ConfigMapLoader {
	return &ConfigMapLoader{
		client:    k8sClient,
		namespace: namespace,
		name:      name,
	}
}

// LoadConfig loads configuration from the ConfigMap
func (l *ConfigMapLoader) LoadConfig(ctx context.Context, config *Config) error {
	// Get the ConfigMap
	configMap := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Namespace: l.namespace,
		Name:      l.name,
	}

	if err := l.client.Get(ctx, key, configMap); err != nil {
		return fmt.Errorf("failed to get ConfigMap %s/%s: %w", l.namespace, l.name, err)
	}

	// Load configuration from ConfigMap data
	if err := l.loadFromData(configMap.Data, config); err != nil {
		return fmt.Errorf("failed to load configuration from ConfigMap: %w", err)
	}

	return nil
}

// loadFromData loads configuration from ConfigMap data
func (l *ConfigMapLoader) loadFromData(data map[string]string, config *Config) error {
	// Check if there's a JSON configuration
	if jsonConfig, exists := data["config.json"]; exists {
		if err := json.Unmarshal([]byte(jsonConfig), config); err != nil {
			return fmt.Errorf("failed to parse JSON configuration: %w", err)
		}
		return nil
	}

	// Load individual configuration values
	l.loadStorageConfig(data, config)
	l.loadHTTPConfig(data, config)
	l.loadRetryConfig(data, config)
	l.loadHooksConfig(data, config)
	l.loadMetricsConfig(data, config)

	return nil
}

// loadStorageConfig loads storage configuration from ConfigMap data
func (l *ConfigMapLoader) loadStorageConfig(data map[string]string, config *Config) {
	if backend, exists := data["storage.backend"]; exists {
		config.Storage.Backend = backend
	}

	// S3 configuration
	if endpoint, exists := data["storage.s3.endpoint"]; exists {
		config.Storage.S3.Endpoint = endpoint
	}
	if bucket, exists := data["storage.s3.bucket"]; exists {
		config.Storage.S3.Bucket = bucket
	}
	if region, exists := data["storage.s3.region"]; exists {
		config.Storage.S3.Region = region
	}
	if accessKeyID, exists := data["storage.s3.accessKeyId"]; exists {
		config.Storage.S3.AccessKeyID = accessKeyID
	}
	if secretAccessKey, exists := data["storage.s3.secretAccessKey"]; exists {
		config.Storage.S3.SecretAccessKey = secretAccessKey
	}
	if useSSLStr, exists := data["storage.s3.useSSL"]; exists {
		if useSSL, err := strconv.ParseBool(useSSLStr); err == nil {
			config.Storage.S3.UseSSL = useSSL
		}
	}
	if pathStyleStr, exists := data["storage.s3.pathStyle"]; exists {
		if pathStyle, err := strconv.ParseBool(pathStyleStr); err == nil {
			config.Storage.S3.PathStyle = pathStyle
		}
	}
}

// loadHTTPConfig loads HTTP configuration from ConfigMap data
func (l *ConfigMapLoader) loadHTTPConfig(data map[string]string, config *Config) {
	if timeoutStr, exists := data["http.timeout"]; exists {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			config.HTTP.Timeout = timeout
		}
	}
	if maxIdleConnsStr, exists := data["http.maxIdleConns"]; exists {
		if maxIdleConns, err := strconv.Atoi(maxIdleConnsStr); err == nil {
			config.HTTP.MaxIdleConns = maxIdleConns
		}
	}
	if maxIdleConnsPerHostStr, exists := data["http.maxIdleConnsPerHost"]; exists {
		if maxIdleConnsPerHost, err := strconv.Atoi(maxIdleConnsPerHostStr); err == nil {
			config.HTTP.MaxIdleConnsPerHost = maxIdleConnsPerHost
		}
	}
	if maxConnsPerHostStr, exists := data["http.maxConnsPerHost"]; exists {
		if maxConnsPerHost, err := strconv.Atoi(maxConnsPerHostStr); err == nil {
			config.HTTP.MaxConnsPerHost = maxConnsPerHost
		}
	}
	if idleConnTimeoutStr, exists := data["http.idleConnTimeout"]; exists {
		if idleConnTimeout, err := time.ParseDuration(idleConnTimeoutStr); err == nil {
			config.HTTP.IdleConnTimeout = idleConnTimeout
		}
	}
	if userAgent, exists := data["http.userAgent"]; exists {
		config.HTTP.UserAgent = userAgent
	}
}

// loadRetryConfig loads retry configuration from ConfigMap data
func (l *ConfigMapLoader) loadRetryConfig(data map[string]string, config *Config) {
	if maxAttemptsStr, exists := data["retry.maxAttempts"]; exists {
		if maxAttempts, err := strconv.Atoi(maxAttemptsStr); err == nil {
			config.Retry.MaxAttempts = maxAttempts
		}
	}
	if baseDelayStr, exists := data["retry.baseDelay"]; exists {
		if baseDelay, err := time.ParseDuration(baseDelayStr); err == nil {
			config.Retry.BaseDelay = baseDelay
		}
	}
	if maxDelayStr, exists := data["retry.maxDelay"]; exists {
		if maxDelay, err := time.ParseDuration(maxDelayStr); err == nil {
			config.Retry.MaxDelay = maxDelay
		}
	}
	if jitterFactorStr, exists := data["retry.jitterFactor"]; exists {
		if jitterFactor, err := strconv.ParseFloat(jitterFactorStr, 64); err == nil {
			config.Retry.JitterFactor = jitterFactor
		}
	}
}

// loadHooksConfig loads hooks configuration from ConfigMap data
func (l *ConfigMapLoader) loadHooksConfig(data map[string]string, config *Config) {
	if whitelistPath, exists := data["hooks.whitelistPath"]; exists {
		config.Hooks.WhitelistPath = whitelistPath
	}
	if endpoint, exists := data["hooks.sidecarEndpoint"]; exists {
		config.Hooks.SidecarEndpoint = endpoint
	}
	if timeoutStr, exists := data["hooks.defaultTimeout"]; exists {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			config.Hooks.DefaultTimeout = timeout
		}
	}
}

// loadMetricsConfig loads metrics configuration from ConfigMap data
func (l *ConfigMapLoader) loadMetricsConfig(data map[string]string, config *Config) {
	if enabledStr, exists := data["metrics.enabled"]; exists {
		if enabled, err := strconv.ParseBool(enabledStr); err == nil {
			config.Metrics.Enabled = enabled
		}
	}
	if intervalStr, exists := data["metrics.interval"]; exists {
		if interval, err := time.ParseDuration(intervalStr); err == nil {
			config.Metrics.Interval = interval
		}
	}
}
