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
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/oddkinco/flux-externalsource-controller/internal/storage"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// Server implements an HTTP server for serving artifacts
type Server struct {
	storage    storage.StorageBackend
	port       int
	httpServer *http.Server
}

// NewServer creates a new artifact HTTP server
func NewServer(backend storage.StorageBackend, port int) *Server {
	return &Server{
		storage: backend,
		port:    port,
	}
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	log := logf.FromContext(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.serveArtifact)

	s.httpServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", s.port),
		Handler:           mux,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Info("Starting artifact HTTP server", "port", s.port)

	// Start server in goroutine
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error(err, "Artifact HTTP server error")
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Shutdown gracefully
	return s.Shutdown(context.Background())
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	log := logf.FromContext(ctx)

	if s.httpServer == nil {
		return nil
	}

	log.Info("Shutting down artifact HTTP server")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return s.httpServer.Shutdown(shutdownCtx)
}

// serveArtifact handles HTTP requests for artifacts
func (s *Server) serveArtifact(w http.ResponseWriter, r *http.Request) {
	log := logf.Log.WithName("artifact-server")

	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract artifact key from URL path
	// Expected format: /artifacts/namespace/name/revision.tar.gz
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		http.Error(w, "Artifact key not specified", http.StatusBadRequest)
		return
	}

	log.V(1).Info("Serving artifact", "key", path, "remote_addr", r.RemoteAddr)

	// Retrieve artifact from storage
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	data, err := s.storage.Retrieve(ctx, path)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.V(1).Info("Artifact not found", "key", path)
			http.Error(w, "Artifact not found", http.StatusNotFound)
			return
		}

		log.Error(err, "Failed to retrieve artifact", "key", path)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set appropriate headers
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", path))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

	// Write artifact data
	if _, err := w.Write(data); err != nil {
		log.Error(err, "Failed to write artifact data", "key", path)
		return
	}

	log.V(1).Info("Successfully served artifact", "key", path, "size", len(data))
}
