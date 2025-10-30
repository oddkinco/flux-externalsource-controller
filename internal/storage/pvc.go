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

package storage

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// PVCBackend implements StorageBackend for PVC-based file storage
// Artifacts are stored as individual files on a persistent volume
type PVCBackend struct {
	basePath string
	baseURL  string
	mutex    sync.RWMutex
}

// NewPVCBackend creates a new PVC storage backend
// basePath is the root directory for storing artifacts (e.g., /data/artifacts)
// baseURL is the URL prefix for accessing artifacts via HTTP
func NewPVCBackend(basePath, baseURL string) (*PVCBackend, error) {
	// Ensure base path exists and is writable
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base path %s: %w", basePath, err)
	}

	// Verify we can write to the directory
	testFile := filepath.Join(basePath, ".write-test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return nil, fmt.Errorf("base path %s is not writable: %w", basePath, err)
	}
	_ = os.Remove(testFile)

	return &PVCBackend{
		basePath: basePath,
		baseURL:  baseURL,
	}, nil
}

// Store saves data to a file and returns the URL
func (p *PVCBackend) Store(ctx context.Context, key string, data []byte) (string, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Validate key to prevent directory traversal
	if err := p.validateKey(key); err != nil {
		return "", err
	}

	// Construct full file path
	filePath := filepath.Join(p.basePath, key)

	// Create parent directories if they don't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write data to file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return p.GetURL(key), nil
}

// List returns a list of keys with the given prefix
func (p *PVCBackend) List(ctx context.Context, prefix string) ([]string, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	var keys []string
	searchPath := filepath.Join(p.basePath, prefix)

	// Check if the search path exists
	if _, err := os.Stat(searchPath); os.IsNotExist(err) {
		return keys, nil // Return empty list if path doesn't exist
	}

	// Walk the directory tree
	err := filepath.WalkDir(searchPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Get relative path from base
		relPath, err := filepath.Rel(p.basePath, path)
		if err != nil {
			return err
		}

		// Convert to forward slashes for consistency
		relPath = filepath.ToSlash(relPath)
		keys = append(keys, relPath)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list files in %s: %w", searchPath, err)
	}

	return keys, nil
}

// Delete removes a file from storage
func (p *PVCBackend) Delete(ctx context.Context, key string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Validate key
	if err := p.validateKey(key); err != nil {
		return err
	}

	filePath := filepath.Join(p.basePath, key)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// File doesn't exist, consider it a successful deletion
		return nil
	}

	// Remove the file
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete file %s: %w", filePath, err)
	}

	// Try to remove empty parent directories (best effort)
	p.cleanupEmptyDirs(filepath.Dir(filePath))

	return nil
}

// GetURL returns the URL for accessing the stored object
func (p *PVCBackend) GetURL(key string) string {
	if p.baseURL != "" {
		return fmt.Sprintf("%s/%s", p.baseURL, key)
	}
	return fmt.Sprintf("file://%s/%s", p.basePath, key)
}

// Retrieve retrieves data from storage by key
func (p *PVCBackend) Retrieve(ctx context.Context, key string) ([]byte, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	// Validate key
	if err := p.validateKey(key); err != nil {
		return nil, err
	}

	filePath := filepath.Join(p.basePath, key)

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("artifact not found: %s", key)
		}
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	return data, nil
}

// validateKey ensures the key doesn't contain path traversal attempts
func (p *PVCBackend) validateKey(key string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	// Check for path traversal attempts
	if strings.Contains(key, "..") {
		return fmt.Errorf("key contains invalid path traversal: %s", key)
	}

	// Ensure key doesn't start with /
	if strings.HasPrefix(key, "/") {
		return fmt.Errorf("key cannot start with /: %s", key)
	}

	return nil
}

// cleanupEmptyDirs removes empty parent directories up to basePath
func (p *PVCBackend) cleanupEmptyDirs(dir string) {
	// Don't remove the base path itself
	if dir == p.basePath || !strings.HasPrefix(dir, p.basePath) {
		return
	}

	// Try to remove the directory (will fail if not empty)
	if err := os.Remove(dir); err == nil {
		// Successfully removed, try parent
		p.cleanupEmptyDirs(filepath.Dir(dir))
	}
}
