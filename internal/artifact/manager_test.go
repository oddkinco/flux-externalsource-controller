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
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/example/externalsource-controller/internal/storage"
)

func TestManager_Package(t *testing.T) {
	tests := []struct {
		name         string
		data         []byte
		path         string
		expectError  bool
		expectedPath string
	}{
		{
			name:         "simple data with path",
			data:         []byte("test data"),
			path:         "config.json",
			expectError:  false,
			expectedPath: "config.json",
		},
		{
			name:         "data with nested path",
			data:         []byte("nested data"),
			path:         "configs/app.yaml",
			expectError:  false,
			expectedPath: "configs/app.yaml",
		},
		{
			name:         "data with empty path defaults to data",
			data:         []byte("default data"),
			path:         "",
			expectError:  false,
			expectedPath: "data",
		},
		{
			name:         "data with dot path defaults to data",
			data:         []byte("dot data"),
			path:         ".",
			expectError:  false,
			expectedPath: "data",
		},
		{
			name:        "invalid path with parent directory",
			data:        []byte("invalid data"),
			path:        "../config.json",
			expectError: true,
		},
		{
			name:         "invalid path with absolute path",
			data:         []byte("absolute data"),
			path:         "/etc/config.json",
			expectError:  false,
			expectedPath: "etc/config.json", // Leading slash should be stripped
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use memory backend for testing
			memStorage := storage.NewMemoryBackend()
			manager := NewManager(memStorage)

			artifact, err := manager.Package(context.Background(), tt.data, tt.path)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify artifact properties
			if artifact == nil {
				t.Fatal("artifact is nil")
			}

			// Verify revision is SHA256 hash
			expectedHash := fmt.Sprintf("%x", sha256.Sum256(tt.data))
			if artifact.Revision != expectedHash {
				t.Errorf("expected revision %s, got %s", expectedHash, artifact.Revision)
			}

			// Verify path
			if artifact.Path != tt.path {
				t.Errorf("expected path %s, got %s", tt.path, artifact.Path)
			}

			// Verify metadata
			if artifact.Metadata == nil {
				t.Error("metadata is nil")
			}

			if artifact.Metadata["contentHash"] != expectedHash {
				t.Errorf("expected contentHash %s, got %s", expectedHash, artifact.Metadata["contentHash"])
			}

			// Verify archive content
			if err := verifyTarGzContent(artifact.Data, tt.data, tt.expectedPath); err != nil {
				t.Errorf("archive verification failed: %v", err)
			}
		})
	}
}

func TestManager_Store(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		data        []byte
		path        string
		expectError bool
	}{
		{
			name:        "successful store",
			source:      "test-source",
			data:        []byte("test data"),
			path:        "config.json",
			expectError: false,
		},
		{
			name:        "store with different source",
			source:      "another-source",
			data:        []byte("other data"),
			path:        "app.yaml",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memStorage := storage.NewMemoryBackend()
			manager := NewManager(memStorage)

			// Package the artifact first
			artifact, err := manager.Package(context.Background(), tt.data, tt.path)
			if err != nil {
				t.Fatalf("failed to package artifact: %v", err)
			}

			// Store the artifact
			url, err := manager.Store(context.Background(), artifact, tt.source)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify URL format
			expectedKey := fmt.Sprintf("artifacts/%s/%s.tar.gz", tt.source, artifact.Revision)
			expectedURL := fmt.Sprintf("memory://localhost/%s", expectedKey)
			if url != expectedURL {
				t.Errorf("expected URL %s, got %s", expectedURL, url)
			}

			// Verify data was stored
			storedData, exists := memStorage.GetData(expectedKey)
			if !exists {
				t.Error("data was not stored")
			}

			if len(storedData) != len(artifact.Data) {
				t.Errorf("stored data length mismatch: expected %d, got %d", len(artifact.Data), len(storedData))
			}
		})
	}
}

func TestManager_Cleanup(t *testing.T) {
	memStorage := storage.NewMemoryBackend()
	manager := NewManager(memStorage)
	ctx := context.Background()

	// Create multiple artifacts for the same source
	source := "test-source"
	artifacts := []struct {
		data []byte
		path string
	}{
		{[]byte("data1"), "config1.json"},
		{[]byte("data2"), "config2.json"},
		{[]byte("data3"), "config3.json"},
	}

	var storedArtifacts []*Artifact
	var urls []string

	// Store all artifacts
	for _, art := range artifacts {
		artifact, err := manager.Package(ctx, art.data, art.path)
		if err != nil {
			t.Fatalf("failed to package artifact: %v", err)
		}

		url, err := manager.Store(ctx, artifact, source)
		if err != nil {
			t.Fatalf("failed to store artifact: %v", err)
		}

		storedArtifacts = append(storedArtifacts, artifact)
		urls = append(urls, url)
	}

	// Verify all artifacts are stored
	if memStorage.Size() != 3 {
		t.Errorf("expected 3 stored artifacts, got %d", memStorage.Size())
	}

	// Keep the second artifact, cleanup others
	keepRevision := storedArtifacts[1].Revision
	err := manager.Cleanup(ctx, source, keepRevision)
	if err != nil {
		t.Errorf("cleanup failed: %v", err)
	}

	// Verify only one artifact remains
	if memStorage.Size() != 1 {
		t.Errorf("expected 1 remaining artifact after cleanup, got %d", memStorage.Size())
	}

	// Verify the correct artifact remains
	keepKey := fmt.Sprintf("artifacts/%s/%s.tar.gz", source, keepRevision)
	_, exists := memStorage.GetData(keepKey)
	if !exists {
		t.Error("kept artifact was deleted")
	}
}

func TestManager_CleanupWithMultipleSources(t *testing.T) {
	memStorage := storage.NewMemoryBackend()
	manager := NewManager(memStorage)
	ctx := context.Background()

	// Create artifacts for different sources
	sources := []string{"source1", "source2"}
	var keepRevisions []string

	for _, source := range sources {
		for i := 0; i < 2; i++ {
			data := []byte(fmt.Sprintf("data-%s-%d", source, i))
			artifact, err := manager.Package(ctx, data, "config.json")
			if err != nil {
				t.Fatalf("failed to package artifact: %v", err)
			}

			_, err = manager.Store(ctx, artifact, source)
			if err != nil {
				t.Fatalf("failed to store artifact: %v", err)
			}

			if i == 1 { // Keep the second artifact for each source
				keepRevisions = append(keepRevisions, artifact.Revision)
			}
		}
	}

	// Verify all artifacts are stored (2 sources × 2 artifacts each = 4 total)
	if memStorage.Size() != 4 {
		t.Errorf("expected 4 stored artifacts, got %d", memStorage.Size())
	}

	// Cleanup first source
	err := manager.Cleanup(ctx, sources[0], keepRevisions[0])
	if err != nil {
		t.Errorf("cleanup failed: %v", err)
	}

	// Should have 3 artifacts now (1 from source1, 2 from source2)
	if memStorage.Size() != 3 {
		t.Errorf("expected 3 artifacts after cleanup, got %d", memStorage.Size())
	}

	// Cleanup second source
	err = manager.Cleanup(ctx, sources[1], keepRevisions[1])
	if err != nil {
		t.Errorf("cleanup failed: %v", err)
	}

	// Should have 2 artifacts now (1 from each source)
	if memStorage.Size() != 2 {
		t.Errorf("expected 2 artifacts after cleanup, got %d", memStorage.Size())
	}
}

// verifyTarGzContent extracts and verifies the content of a tar.gz archive
func verifyTarGzContent(archiveData, expectedData []byte, expectedPath string) error {
	// Create gzip reader
	gzReader, err := gzip.NewReader(strings.NewReader(string(archiveData)))
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	// Read the first (and should be only) entry
	header, err := tarReader.Next()
	if err != nil {
		return fmt.Errorf("failed to read tar header: %w", err)
	}

	// Verify the file name
	if header.Name != expectedPath {
		return fmt.Errorf("expected file name %s, got %s", expectedPath, header.Name)
	}

	// Verify the file size
	if header.Size != int64(len(expectedData)) {
		return fmt.Errorf("expected file size %d, got %d", len(expectedData), header.Size)
	}

	// Read and verify the content
	content, err := io.ReadAll(tarReader)
	if err != nil {
		return fmt.Errorf("failed to read file content: %w", err)
	}

	if string(content) != string(expectedData) {
		return fmt.Errorf("content mismatch: expected %s, got %s", string(expectedData), string(content))
	}

	// Verify there are no more entries
	_, err = tarReader.Next()
	if err != io.EOF {
		return fmt.Errorf("expected EOF, but found more entries")
	}

	return nil
}
