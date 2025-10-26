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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	sourcev1alpha1 "github.com/oddkinco/flux-externalsource-controller/api/v1alpha1"
)

// ExecuteRequest represents the request to the sidecar
type ExecuteRequest struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Timeout string            `json:"timeout"`
	Env     map[string]string `json:"env"`
	Stdin   string            `json:"stdin"` // base64 encoded
}

// ExecuteResponse represents the response from the sidecar
type ExecuteResponse struct {
	Stdout   string `json:"stdout"` // base64 encoded
	Stderr   string `json:"stderr"` // base64 encoded
	ExitCode int    `json:"exitCode"`
}

// SidecarExecutor implements HookExecutor by communicating with a sidecar container
type SidecarExecutor struct {
	endpoint         string
	httpClient       *http.Client
	whitelistManager WhitelistManager
	defaultTimeout   time.Duration
}

// NewSidecarExecutor creates a new sidecar hook executor
func NewSidecarExecutor(endpoint string, whitelistManager WhitelistManager, defaultTimeout time.Duration) *SidecarExecutor {
	return &SidecarExecutor{
		endpoint:         endpoint,
		httpClient:       &http.Client{Timeout: 5 * time.Minute}, // Overall HTTP timeout
		whitelistManager: whitelistManager,
		defaultTimeout:   defaultTimeout,
	}
}

// Execute runs a hook with the given input data
func (s *SidecarExecutor) Execute(ctx context.Context, input []byte, hook sourcev1alpha1.HookSpec) ([]byte, error) {
	// Validate command against whitelist
	if !s.whitelistManager.IsAllowed(hook.Command, hook.Args) {
		return nil, fmt.Errorf("command %s with args %v is not whitelisted", hook.Command, hook.Args)
	}

	// Parse timeout
	timeout := s.defaultTimeout
	if hook.Timeout != "" {
		parsedTimeout, err := time.ParseDuration(hook.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout %s: %w", hook.Timeout, err)
		}
		timeout = parsedTimeout
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Prepare environment variables
	env := make(map[string]string)
	for _, envVar := range hook.Env {
		env[envVar.Name] = envVar.Value
	}

	// Prepare request
	req := ExecuteRequest{
		Command: hook.Command,
		Args:    hook.Args,
		Timeout: hook.Timeout,
		Env:     env,
		Stdin:   base64.StdEncoding.EncodeToString(input),
	}

	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(execCtx, "POST", s.endpoint+"/execute", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to sidecar: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sidecar returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var execResp ExecuteResponse
	if err := json.Unmarshal(respBody, &execResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check exit code
	if execResp.ExitCode != 0 {
		stderr, _ := base64.StdEncoding.DecodeString(execResp.Stderr)
		return nil, fmt.Errorf("command exited with code %d: %s", execResp.ExitCode, string(stderr))
	}

	// Decode stdout
	output, err := base64.StdEncoding.DecodeString(execResp.Stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to decode stdout: %w", err)
	}

	return output, nil
}
