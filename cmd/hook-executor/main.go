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

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/oddkinco/flux-externalsource-controller/internal/hooks"
)

var (
	port          = flag.Int("port", 8081, "Port to listen on")
	whitelistPath = flag.String("whitelist", "/etc/hooks/whitelist.yaml", "Path to whitelist configuration file")
)

// ExecuteRequest represents the request to execute a command
type ExecuteRequest struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Timeout string            `json:"timeout"`
	Env     map[string]string `json:"env"`
	Stdin   string            `json:"stdin"` // base64 encoded
}

// ExecuteResponse represents the response from command execution
type ExecuteResponse struct {
	Stdout   string `json:"stdout"` // base64 encoded
	Stderr   string `json:"stderr"` // base64 encoded
	ExitCode int    `json:"exitCode"`
}

// Server handles hook execution requests
type Server struct {
	whitelistManager hooks.WhitelistManager
}

// NewServer creates a new hook executor server
func NewServer(whitelistManager hooks.WhitelistManager) *Server {
	return &Server{
		whitelistManager: whitelistManager,
	}
}

// handleExecute handles the /execute endpoint
func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse request: %v", err), http.StatusBadRequest)
		return
	}

	// Validate command against whitelist
	if !s.whitelistManager.IsAllowed(req.Command, req.Args) {
		log.Printf("Command not whitelisted: %s %v", req.Command, req.Args)
		http.Error(w, fmt.Sprintf("Command %s is not whitelisted", req.Command), http.StatusForbidden)
		return
	}

	// Parse timeout
	timeout := 30 * time.Second
	if req.Timeout != "" {
		parsedTimeout, err := time.ParseDuration(req.Timeout)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid timeout: %v", err), http.StatusBadRequest)
			return
		}
		timeout = parsedTimeout
	}

	// Decode stdin
	var stdin []byte
	if req.Stdin != "" {
		var err error
		stdin, err = base64.StdEncoding.DecodeString(req.Stdin)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to decode stdin: %v", err), http.StatusBadRequest)
			return
		}
	}

	// Execute command
	resp := s.executeCommand(r.Context(), req.Command, req.Args, stdin, req.Env, timeout)

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// executeCommand executes a command with the given parameters
func (s *Server) executeCommand(ctx context.Context, command string, args []string, stdin []byte, env map[string]string, timeout time.Duration) ExecuteResponse {
	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create command
	cmd := exec.CommandContext(execCtx, command, args...)

	// Set up stdin
	if len(stdin) > 0 {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	// Set up stdout and stderr buffers
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set environment variables
	if len(env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Execute command
	err := cmd.Run()

	// Determine exit code
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Command failed to start or other error
			exitCode = 1
			stderr.WriteString(fmt.Sprintf("\nExecution error: %v", err))
		}
	}

	// Encode outputs
	return ExecuteResponse{
		Stdout:   base64.StdEncoding.EncodeToString(stdout.Bytes()),
		Stderr:   base64.StdEncoding.EncodeToString(stderr.Bytes()),
		ExitCode: exitCode,
	}
}

// handleHealth handles the /health endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "healthy"}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func main() {
	flag.Parse()

	log.Printf("Starting hook-executor server on port %d", *port)
	log.Printf("Loading whitelist from %s", *whitelistPath)

	// Load whitelist
	whitelistManager, err := hooks.NewFileWhitelistManager(*whitelistPath)
	if err != nil {
		log.Fatalf("Failed to load whitelist: %v", err)
	}

	// Create server
	server := NewServer(whitelistManager)

	// Set up HTTP handlers
	http.HandleFunc("/execute", server.handleExecute)
	http.HandleFunc("/health", server.handleHealth)

	// Start server
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
