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
	"net/http"
	"net/http/httptest"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestHTTPGenerator_SupportsConditionalFetch(t *testing.T) {
	generator := NewHTTPGenerator(nil)
	if !generator.SupportsConditionalFetch() {
		t.Error("HTTP generator should support conditional fetch")
	}
}

func TestHTTPGenerator_Generate_Success(t *testing.T) {
	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", "test-etag")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "test data"}`))
	}))
	defer server.Close()

	generator := NewHTTPGenerator(nil)
	config := GeneratorConfig{
		Type: "http",
		Config: map[string]interface{}{
			"url":    server.URL,
			"method": "GET",
		},
	}

	ctx := context.Background()
	data, err := generator.Generate(ctx, config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if string(data.Data) != `{"message": "test data"}` {
		t.Errorf("Expected test data, got %s", string(data.Data))
	}

	if data.LastModified != "test-etag" {
		t.Errorf("Expected test-etag, got %s", data.LastModified)
	}

	if data.Metadata["content-type"] != "application/json" {
		t.Errorf("Expected application/json content-type, got %s", data.Metadata["content-type"])
	}
}

func TestHTTPGenerator_Generate_HTTPError(t *testing.T) {
	// Create test HTTP server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	generator := NewHTTPGenerator(nil)
	config := GeneratorConfig{
		Type: "http",
		Config: map[string]interface{}{
			"url": server.URL,
		},
	}

	ctx := context.Background()
	_, err := generator.Generate(ctx, config)
	if err == nil {
		t.Error("Expected error for HTTP 500 response")
	}
}

func TestHTTPGenerator_Generate_InvalidURL(t *testing.T) {
	generator := NewHTTPGenerator(nil)
	config := GeneratorConfig{
		Type: "http",
		Config: map[string]interface{}{
			"url": "invalid-url",
		},
	}

	ctx := context.Background()
	_, err := generator.Generate(ctx, config)
	if err == nil {
		t.Error("Expected error for invalid URL")
	}
}

func TestHTTPGenerator_GetLastModified_Success(t *testing.T) {
	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			t.Errorf("Expected HEAD request, got %s", r.Method)
		}
		w.Header().Set("ETag", "test-etag")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	generator := NewHTTPGenerator(nil)
	config := GeneratorConfig{
		Type: "http",
		Config: map[string]interface{}{
			"url": server.URL,
		},
	}

	ctx := context.Background()
	etag, err := generator.GetLastModified(ctx, config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if etag != "test-etag" {
		t.Errorf("Expected test-etag, got %s", etag)
	}
}

func TestHTTPGenerator_GetLastModified_HTTPError(t *testing.T) {
	// Create test HTTP server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	generator := NewHTTPGenerator(nil)
	config := GeneratorConfig{
		Type: "http",
		Config: map[string]interface{}{
			"url": server.URL,
		},
	}

	ctx := context.Background()
	_, err := generator.GetLastModified(ctx, config)
	if err == nil {
		t.Error("Expected error for HTTP 404 response")
	}
}

func TestHTTPGenerator_ParseConfig_MissingURL(t *testing.T) {
	generator := NewHTTPGenerator(nil)
	config := map[string]interface{}{
		"method": "GET",
	}

	ctx := context.Background()
	_, err := generator.parseConfig(ctx, config)
	if err == nil {
		t.Error("Expected error for missing URL")
	}
}

func TestHTTPGenerator_ParseConfig_WithMethod(t *testing.T) {
	generator := NewHTTPGenerator(nil)
	config := map[string]interface{}{
		"url":    "https://example.com",
		"method": "POST",
	}

	ctx := context.Background()
	httpConfig, err := generator.parseConfig(ctx, config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if httpConfig.Method != "POST" {
		t.Errorf("Expected POST method, got %s", httpConfig.Method)
	}
}

func TestHTTPGenerator_ParseConfig_DefaultMethod(t *testing.T) {
	generator := NewHTTPGenerator(nil)
	config := map[string]interface{}{
		"url": "https://example.com",
	}

	ctx := context.Background()
	httpConfig, err := generator.parseConfig(ctx, config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if httpConfig.Method != "GET" {
		t.Errorf("Expected GET method by default, got %s", httpConfig.Method)
	}
}

func TestHTTPGenerator_ParseConfig_InsecureSkipVerify(t *testing.T) {
	generator := NewHTTPGenerator(nil)
	config := map[string]interface{}{
		"url":                "https://example.com",
		"insecureSkipVerify": true,
	}

	ctx := context.Background()
	httpConfig, err := generator.parseConfig(ctx, config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !httpConfig.InsecureSkipVerify {
		t.Error("Expected insecureSkipVerify to be true")
	}
}

func TestHTTPGenerator_LoadHeaders_Success(t *testing.T) {
	// Create fake Kubernetes client with secret
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-headers",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"Authorization": []byte("Bearer token123"),
			"X-Custom":      []byte("custom-value"),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	generator := NewHTTPGenerator(fakeClient)

	ctx := context.Background()
	headers, err := generator.loadHeaders(ctx, "default", "test-headers")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if headers["Authorization"] != "Bearer token123" {
		t.Errorf("Expected Authorization header, got %s", headers["Authorization"])
	}

	if headers["X-Custom"] != "custom-value" {
		t.Errorf("Expected X-Custom header, got %s", headers["X-Custom"])
	}
}

func TestHTTPGenerator_LoadHeaders_SecretNotFound(t *testing.T) {
	// Create fake Kubernetes client without secret
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	generator := NewHTTPGenerator(fakeClient)

	ctx := context.Background()
	_, err := generator.loadHeaders(ctx, "default", "nonexistent-secret")
	if err == nil {
		t.Error("Expected error for nonexistent secret")
	}
}

func TestHTTPGenerator_LoadSecretData_Success(t *testing.T) {
	// Create fake Kubernetes client with secret
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ca",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"ca.crt": []byte("-----BEGIN CERTIFICATE-----\ntest-ca-data\n-----END CERTIFICATE-----"),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	generator := NewHTTPGenerator(fakeClient)

	ctx := context.Background()
	caData, err := generator.loadSecretData(ctx, "default", "test-ca", "ca.crt")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := "-----BEGIN CERTIFICATE-----\ntest-ca-data\n-----END CERTIFICATE-----"
	if string(caData) != expected {
		t.Errorf("Expected CA data, got %s", string(caData))
	}
}

func TestHTTPGenerator_LoadSecretData_KeyNotFound(t *testing.T) {
	// Create fake Kubernetes client with secret
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ca",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"ca.crt": []byte("test-ca-data"),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	generator := NewHTTPGenerator(fakeClient)

	ctx := context.Background()
	_, err := generator.loadSecretData(ctx, "default", "test-ca", "nonexistent-key")
	if err == nil {
		t.Error("Expected error for nonexistent key")
	}
}
