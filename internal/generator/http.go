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

package generator

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HTTPGenerator implements SourceGenerator for HTTP sources
type HTTPGenerator struct {
	client    client.Client
	httpClient *http.Client
}

// HTTPConfig holds HTTP-specific configuration
type HTTPConfig struct {
	URL                string            `json:"url"`
	Method             string            `json:"method"`
	Headers            map[string]string `json:"headers"`
	CABundle           []byte            `json:"caBundle"`
	InsecureSkipVerify bool              `json:"insecureSkipVerify"`
	Timeout            time.Duration     `json:"timeout"`
}

// NewHTTPGenerator creates a new HTTP generator
func NewHTTPGenerator(client client.Client) *HTTPGenerator {
	return &HTTPGenerator{
		client: client,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Generate fetches data from the HTTP endpoint
func (h *HTTPGenerator) Generate(ctx context.Context, config GeneratorConfig) (*SourceData, error) {
	httpConfig, err := h.parseConfig(ctx, config.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTTP config: %w", err)
	}

	// Configure HTTP client with TLS settings
	client, err := h.configureHTTPClient(ctx, httpConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to configure HTTP client: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, httpConfig.Method, httpConfig.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add headers
	for key, value := range httpConfig.Headers {
		req.Header.Set(key, value)
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	// Read response body
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Extract ETag for conditional fetching
	etag := resp.Header.Get("ETag")

	return &SourceData{
		Data:         data,
		LastModified: etag,
		Metadata: map[string]string{
			"content-type":   resp.Header.Get("Content-Type"),
			"content-length": resp.Header.Get("Content-Length"),
			"etag":           etag,
		},
	}, nil
}

// SupportsConditionalFetch returns true as HTTP supports ETag-based conditional fetching
func (h *HTTPGenerator) SupportsConditionalFetch() bool {
	return true
}

// GetLastModified performs a HEAD request to get the current ETag
func (h *HTTPGenerator) GetLastModified(ctx context.Context, config GeneratorConfig) (string, error) {
	httpConfig, err := h.parseConfig(ctx, config.Config)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTTP config: %w", err)
	}

	// Configure HTTP client with TLS settings
	client, err := h.configureHTTPClient(ctx, httpConfig)
	if err != nil {
		return "", fmt.Errorf("failed to configure HTTP client: %w", err)
	}

	// Create HEAD request
	req, err := http.NewRequestWithContext(ctx, "HEAD", httpConfig.URL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create HEAD request: %w", err)
	}

	// Add headers
	for key, value := range httpConfig.Headers {
		req.Header.Set(key, value)
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HEAD request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HEAD request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	return resp.Header.Get("ETag"), nil
}

// parseConfig converts the generic config map to HTTPConfig
func (h *HTTPGenerator) parseConfig(ctx context.Context, config map[string]interface{}) (*HTTPConfig, error) {
	httpConfig := &HTTPConfig{
		Method:  "GET",
		Headers: make(map[string]string),
		Timeout: 30 * time.Second,
	}

	// Parse URL
	if url, ok := config["url"].(string); ok {
		httpConfig.URL = url
	} else {
		return nil, fmt.Errorf("url is required and must be a string")
	}

	// Parse method
	if method, ok := config["method"].(string); ok && method != "" {
		httpConfig.Method = method
	}

	// Parse insecureSkipVerify
	if insecure, ok := config["insecureSkipVerify"].(bool); ok {
		httpConfig.InsecureSkipVerify = insecure
	}

	// Parse namespace for secret references
	namespace, _ := config["namespace"].(string)
	if namespace == "" {
		namespace = "default"
	}

	// Load headers from secret if specified
	if headersSecretName, ok := config["headersSecretName"].(string); ok && headersSecretName != "" {
		headers, err := h.loadHeaders(ctx, namespace, headersSecretName)
		if err != nil {
			return nil, fmt.Errorf("failed to load headers from secret: %w", err)
		}
		for k, v := range headers {
			httpConfig.Headers[k] = v
		}
	}

	// Load CA bundle from secret if specified
	if caBundleSecretName, ok := config["caBundleSecretName"].(string); ok && caBundleSecretName != "" {
		caBundleKey, _ := config["caBundleSecretKey"].(string)
		if caBundleKey == "" {
			caBundleKey = "ca.crt"
		}
		
		caBundle, err := h.loadSecretData(ctx, namespace, caBundleSecretName, caBundleKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load CA bundle from secret: %w", err)
		}
		httpConfig.CABundle = caBundle
	}

	return httpConfig, nil
}

// configureHTTPClient creates an HTTP client with appropriate TLS configuration
func (h *HTTPGenerator) configureHTTPClient(ctx context.Context, config *HTTPConfig) (*http.Client, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.InsecureSkipVerify,
		},
	}

	// Configure custom CA bundle if provided
	if len(config.CABundle) > 0 {
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(config.CABundle) {
			return nil, fmt.Errorf("failed to parse CA bundle")
		}
		transport.TLSClientConfig.RootCAs = caCertPool
	}

	return &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}, nil
}

// loadSecretData loads data from a Kubernetes secret
func (h *HTTPGenerator) loadSecretData(ctx context.Context, namespace, name, key string) ([]byte, error) {
	secret := &corev1.Secret{}
	secretKey := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	if err := h.client.Get(ctx, secretKey, secret); err != nil {
		return nil, fmt.Errorf("failed to get secret %s/%s: %w", namespace, name, err)
	}

	data, exists := secret.Data[key]
	if !exists {
		return nil, fmt.Errorf("key %s not found in secret %s/%s", key, namespace, name)
	}

	return data, nil
}

// loadHeaders loads headers from a Kubernetes secret
func (h *HTTPGenerator) loadHeaders(ctx context.Context, namespace, secretName string) (map[string]string, error) {
	secret := &corev1.Secret{}
	secretKey := client.ObjectKey{
		Namespace: namespace,
		Name:      secretName,
	}

	if err := h.client.Get(ctx, secretKey, secret); err != nil {
		return nil, fmt.Errorf("failed to get headers secret %s/%s: %w", namespace, secretName, err)
	}

	headers := make(map[string]string)
	for key, value := range secret.Data {
		headers[key] = string(value)
	}

	return headers, nil
}