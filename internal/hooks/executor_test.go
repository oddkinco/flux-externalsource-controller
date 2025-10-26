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

package hooks

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sourcev1alpha1 "github.com/oddkinco/flux-externalsource-controller/api/v1alpha1"
)

// mockWhitelistManager is a mock implementation for testing
type mockWhitelistManager struct {
	allowed bool
}

func (m *mockWhitelistManager) IsAllowed(command string, args []string) bool {
	return m.allowed
}

func (m *mockWhitelistManager) Reload() error {
	return nil
}

func TestSidecarExecutor_Execute(t *testing.T) {
	tests := []struct {
		name          string
		hook          sourcev1alpha1.HookSpec
		input         []byte
		whitelisted   bool
		serverHandler func(w http.ResponseWriter, r *http.Request)
		wantErr       bool
		wantOutput    []byte
	}{
		{
			name: "successful execution",
			hook: sourcev1alpha1.HookSpec{
				Name:        "test-hook",
				Command:     "jq",
				Args:        []string{".field"},
				Timeout:     "30s",
				RetryPolicy: "fail",
			},
			input:       []byte(`{"field": "value"}`),
			whitelisted: true,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				// Validate request
				if r.Method != "POST" {
					t.Errorf("Expected POST request, got %s", r.Method)
				}
				if r.URL.Path != "/execute" {
					t.Errorf("Expected /execute path, got %s", r.URL.Path)
				}

				// Read and validate request body
				var req ExecuteRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("Failed to decode request: %v", err)
				}

				// Send response
				resp := ExecuteResponse{
					Stdout:   base64.StdEncoding.EncodeToString([]byte("output")),
					Stderr:   "",
					ExitCode: 0,
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(resp); err != nil {
					http.Error(w, "Failed to encode response", http.StatusInternalServerError)
				}
			},
			wantErr:    false,
			wantOutput: []byte("output"),
		},
		{
			name: "command not whitelisted",
			hook: sourcev1alpha1.HookSpec{
				Name:    "test-hook",
				Command: "rm",
				Args:    []string{"-rf", "/"},
			},
			input:       []byte("test"),
			whitelisted: false,
			wantErr:     true,
		},
		{
			name: "command exits with non-zero code",
			hook: sourcev1alpha1.HookSpec{
				Name:        "test-hook",
				Command:     "jq",
				Args:        []string{".invalid"},
				Timeout:     "30s",
				RetryPolicy: "fail",
			},
			input:       []byte(`{}`),
			whitelisted: true,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				resp := ExecuteResponse{
					Stdout:   "",
					Stderr:   base64.StdEncoding.EncodeToString([]byte("error message")),
					ExitCode: 1,
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(resp); err != nil {
					http.Error(w, "Failed to encode response", http.StatusInternalServerError)
				}
			},
			wantErr: true,
		},
		{
			name: "with environment variables",
			hook: sourcev1alpha1.HookSpec{
				Name:        "test-hook",
				Command:     "env-test",
				Args:        []string{},
				Timeout:     "30s",
				RetryPolicy: "fail",
				Env: []sourcev1alpha1.EnvVar{
					{Name: "FOO", Value: "bar"},
					{Name: "BAZ", Value: "qux"},
				},
			},
			input:       []byte("test"),
			whitelisted: true,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				var req ExecuteRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("Failed to decode request: %v", err)
				}

				// Validate environment variables
				if req.Env["FOO"] != "bar" {
					t.Errorf("Expected FOO=bar, got %s", req.Env["FOO"])
				}
				if req.Env["BAZ"] != "qux" {
					t.Errorf("Expected BAZ=qux, got %s", req.Env["BAZ"])
				}

				resp := ExecuteResponse{
					Stdout:   base64.StdEncoding.EncodeToString([]byte("success")),
					Stderr:   "",
					ExitCode: 0,
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(resp); err != nil {
					http.Error(w, "Failed to encode response", http.StatusInternalServerError)
				}
			},
			wantErr:    false,
			wantOutput: []byte("success"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			var server *httptest.Server
			if tt.serverHandler != nil {
				server = httptest.NewServer(http.HandlerFunc(tt.serverHandler))
				defer server.Close()
			}

			// Create mock whitelist manager
			wm := &mockWhitelistManager{allowed: tt.whitelisted}

			// Create executor
			var endpoint string
			if server != nil {
				endpoint = server.URL
			} else {
				endpoint = "http://localhost:9999" // Invalid endpoint for error cases
			}
			executor := NewSidecarExecutor(endpoint, wm, 30*time.Second)

			// Execute hook
			ctx := context.Background()
			output, err := executor.Execute(ctx, tt.input, tt.hook)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check output
			if !tt.wantErr && string(output) != string(tt.wantOutput) {
				t.Errorf("Execute() output = %s, want %s", string(output), string(tt.wantOutput))
			}
		})
	}
}

func TestSidecarExecutor_Timeout(t *testing.T) {
	// Create server that sleeps longer than timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		resp := ExecuteResponse{
			Stdout:   base64.StdEncoding.EncodeToString([]byte("too late")),
			Stderr:   "",
			ExitCode: 0,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	wm := &mockWhitelistManager{allowed: true}
	executor := NewSidecarExecutor(server.URL, wm, 1*time.Second)

	hook := sourcev1alpha1.HookSpec{
		Name:    "timeout-test",
		Command: "sleep",
		Args:    []string{"10"},
		Timeout: "100ms", // Very short timeout
	}

	ctx := context.Background()
	_, err := executor.Execute(ctx, []byte("test"), hook)

	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}
