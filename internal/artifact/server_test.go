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

package artifact

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/oddkinco/flux-externalsource-controller/internal/storage"
)

func TestServer_ServeArtifact(t *testing.T) {
	// Create memory backend and store test data
	backend := storage.NewMemoryBackend()
	ctx := context.Background()

	testKey := "artifacts/namespace/name/abc123.tar.gz"
	testData := []byte("test artifact data")

	_, err := backend.Store(ctx, testKey, testData)
	if err != nil {
		t.Fatalf("failed to store test data: %v", err)
	}

	// Create server
	server := NewServer(backend, 8080)

	tests := []struct {
		name           string
		path           string
		method         string
		expectedStatus int
		expectedData   []byte
	}{
		{
			name:           "successful GET",
			path:           "/" + testKey,
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedData:   testData,
		},
		{
			name:           "not found",
			path:           "/artifacts/nonexistent.tar.gz",
			method:         http.MethodGet,
			expectedStatus: http.StatusNotFound,
			expectedData:   nil,
		},
		{
			name:           "method not allowed",
			path:           "/" + testKey,
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedData:   nil,
		},
		{
			name:           "empty path",
			path:           "/",
			method:         http.MethodGet,
			expectedStatus: http.StatusBadRequest,
			expectedData:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			server.serveArtifact(w, req)

			resp := w.Result()
			defer func() {
				if err := resp.Body.Close(); err != nil { //nolint:staticcheck // SA9003: Intentionally empty
					// Ignore close errors in tests
				}
			}()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			if tt.expectedData != nil {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("failed to read response body: %v", err)
				}

				if string(body) != string(tt.expectedData) {
					t.Errorf("expected data %s, got %s", string(tt.expectedData), string(body))
				}

				// Verify headers
				contentType := resp.Header.Get("Content-Type")
				if contentType != "application/gzip" {
					t.Errorf("expected Content-Type application/gzip, got %s", contentType)
				}
			}
		})
	}
}

func TestServer_Shutdown(t *testing.T) {
	backend := storage.NewMemoryBackend()
	server := NewServer(backend, 0) // Use port 0 for automatic assignment

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Server should start without error
	go func() {
		_ = server.Start(ctx)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown should complete without error
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

func TestServer_ShutdownWithoutStart(t *testing.T) {
	backend := storage.NewMemoryBackend()
	server := NewServer(backend, 8080)

	// Shutdown without starting should not error
	ctx := context.Background()
	if err := server.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}
