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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/example/externalsource-controller/internal/storage"
)

// Manager implements the ArtifactManager interface
type Manager struct {
	storage storage.StorageBackend
}

// NewManager creates a new artifact manager with the given storage backend
func NewManager(storage storage.StorageBackend) *Manager {
	return &Manager{
		storage: storage,
	}
}

// Package creates a .tar.gz archive from the given data and calculates SHA256 digest
func (m *Manager) Package(ctx context.Context, data []byte, path string) (*Artifact, error) {
	// Calculate SHA256 digest for content-based versioning
	hash := sha256.Sum256(data)
	revision := fmt.Sprintf("%x", hash)

	// Create .tar.gz archive
	archiveData, err := m.createTarGzArchive(data, path)
	if err != nil {
		return nil, fmt.Errorf("failed to create tar.gz archive: %w", err)
	}

	artifact := &Artifact{
		Data:     archiveData,
		Path:     path,
		Revision: revision,
		Metadata: map[string]string{
			"created":     time.Now().UTC().Format(time.RFC3339),
			"size":        fmt.Sprintf("%d", len(archiveData)),
			"contentHash": revision,
		},
	}

	return artifact, nil
}

// Store uploads the artifact to the storage backend and returns the URL
func (m *Manager) Store(ctx context.Context, artifact *Artifact, source string) (string, error) {
	// Generate source-specific storage key based on revision
	key := fmt.Sprintf("artifacts/%s/%s.tar.gz", source, artifact.Revision)

	// Upload to storage backend
	url, err := m.storage.Store(ctx, key, artifact.Data)
	if err != nil {
		return "", fmt.Errorf("failed to store artifact: %w", err)
	}

	return url, nil
}

// Cleanup removes obsolete artifacts, keeping only the specified revision
func (m *Manager) Cleanup(ctx context.Context, source string, keepRevision string) error {
	// Use source-specific prefix to avoid affecting other sources
	prefix := fmt.Sprintf("artifacts/%s/", source)
	keys, err := m.storage.List(ctx, prefix)
	if err != nil {
		return fmt.Errorf("failed to list artifacts for cleanup: %w", err)
	}

	// Delete all artifacts except the one we want to keep
	keepKey := fmt.Sprintf("artifacts/%s/%s.tar.gz", source, keepRevision)
	var cleanupErrors []error

	for _, key := range keys {
		if key != keepKey {
			if err := m.storage.Delete(ctx, key); err != nil {
				// Collect errors but continue cleanup
				cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to delete %s: %w", key, err))
				continue
			}
		}
	}

	// Return aggregated errors if any occurred
	if len(cleanupErrors) > 0 {
		return fmt.Errorf("cleanup completed with %d errors: %v", len(cleanupErrors), cleanupErrors)
	}

	return nil
}

// createTarGzArchive creates a .tar.gz archive with proper directory structure
func (m *Manager) createTarGzArchive(data []byte, destinationPath string) ([]byte, error) {
	var buf bytes.Buffer

	// Create gzip writer
	gzWriter := gzip.NewWriter(&buf)
	defer gzWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Normalize the destination path
	cleanPath := filepath.Clean(destinationPath)
	if cleanPath == "." || cleanPath == "/" {
		cleanPath = "data"
	}

	// Ensure path doesn't start with / or contain ..
	cleanPath = strings.TrimPrefix(cleanPath, "/")
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("invalid destination path: %s", destinationPath)
	}

	// Create tar header
	header := &tar.Header{
		Name:    cleanPath,
		Mode:    0644,
		Size:    int64(len(data)),
		ModTime: time.Now(),
	}

	// Write header
	if err := tarWriter.WriteHeader(header); err != nil {
		return nil, fmt.Errorf("failed to write tar header: %w", err)
	}

	// Write data
	if _, err := tarWriter.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write data to tar: %w", err)
	}

	// Close writers to flush data
	if err := tarWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close tar writer: %w", err)
	}

	if err := gzWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}
