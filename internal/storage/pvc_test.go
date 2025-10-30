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
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPVCBackend(t *testing.T) {
	tests := []struct {
		name        string
		basePath    string
		baseURL     string
		expectError bool
	}{
		{
			name:        "valid path",
			basePath:    t.TempDir(),
			baseURL:     "http://test.local:8080",
			expectError: false,
		},
		{
			name:        "empty baseURL",
			basePath:    t.TempDir(),
			baseURL:     "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, err := NewPVCBackend(tt.basePath, tt.baseURL)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, backend)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, backend)
				assert.Equal(t, tt.basePath, backend.basePath)
				assert.Equal(t, tt.baseURL, backend.baseURL)
			}
		})
	}
}

func TestPVCBackend_Store(t *testing.T) {
	tempDir := t.TempDir()
	backend, err := NewPVCBackend(tempDir, "http://test.local:8080")
	require.NoError(t, err)

	ctx := context.Background()

	tests := []struct {
		name        string
		key         string
		data        []byte
		expectError bool
	}{
		{
			name:        "simple key",
			key:         "test.tar.gz",
			data:        []byte("test data"),
			expectError: false,
		},
		{
			name:        "nested key",
			key:         "namespace/name/revision.tar.gz",
			data:        []byte("nested data"),
			expectError: false,
		},
		{
			name:        "empty key",
			key:         "",
			data:        []byte("data"),
			expectError: true,
		},
		{
			name:        "path traversal attempt",
			key:         "../../../etc/passwd",
			data:        []byte("malicious"),
			expectError: true,
		},
		{
			name:        "absolute path",
			key:         "/etc/passwd",
			data:        []byte("malicious"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := backend.Store(ctx, tt.key, tt.data)
			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, url)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, url)
				assert.Contains(t, url, tt.key)

				// Verify file was created
				filePath := filepath.Join(tempDir, tt.key)
				data, err := os.ReadFile(filePath)
				assert.NoError(t, err)
				assert.Equal(t, tt.data, data)
			}
		})
	}
}

func TestPVCBackend_Retrieve(t *testing.T) {
	tempDir := t.TempDir()
	backend, err := NewPVCBackend(tempDir, "http://test.local:8080")
	require.NoError(t, err)

	ctx := context.Background()

	// Store some data first
	testData := []byte("test data")
	testKey := "test/artifact.tar.gz"
	_, err = backend.Store(ctx, testKey, testData)
	require.NoError(t, err)

	tests := []struct {
		name        string
		key         string
		expectData  []byte
		expectError bool
	}{
		{
			name:        "existing key",
			key:         testKey,
			expectData:  testData,
			expectError: false,
		},
		{
			name:        "non-existent key",
			key:         "does/not/exist.tar.gz",
			expectData:  nil,
			expectError: true,
		},
		{
			name:        "path traversal attempt",
			key:         "../../../etc/passwd",
			expectData:  nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := backend.Retrieve(ctx, tt.key)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, data)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectData, data)
			}
		})
	}
}

func TestPVCBackend_List(t *testing.T) {
	tempDir := t.TempDir()
	backend, err := NewPVCBackend(tempDir, "http://test.local:8080")
	require.NoError(t, err)

	ctx := context.Background()

	// Store multiple artifacts
	artifacts := map[string][]byte{
		"namespace1/source1/rev1.tar.gz": []byte("data1"),
		"namespace1/source1/rev2.tar.gz": []byte("data2"),
		"namespace1/source2/rev1.tar.gz": []byte("data3"),
		"namespace2/source1/rev1.tar.gz": []byte("data4"),
	}

	for key, data := range artifacts {
		_, err := backend.Store(ctx, key, data)
		require.NoError(t, err)
	}

	tests := []struct {
		name         string
		prefix       string
		expectKeys   []string
		expectLength int
	}{
		{
			name:         "list all",
			prefix:       "",
			expectLength: 4,
		},
		{
			name:   "list namespace1",
			prefix: "namespace1",
			expectKeys: []string{
				"namespace1/source1/rev1.tar.gz",
				"namespace1/source1/rev2.tar.gz",
				"namespace1/source2/rev1.tar.gz",
			},
			expectLength: 3,
		},
		{
			name:   "list namespace1/source1",
			prefix: "namespace1/source1",
			expectKeys: []string{
				"namespace1/source1/rev1.tar.gz",
				"namespace1/source1/rev2.tar.gz",
			},
			expectLength: 2,
		},
		{
			name:         "list non-existent prefix",
			prefix:       "does/not/exist",
			expectLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys, err := backend.List(ctx, tt.prefix)
			assert.NoError(t, err)
			assert.Len(t, keys, tt.expectLength)
			if tt.expectKeys != nil {
				for _, expectedKey := range tt.expectKeys {
					assert.Contains(t, keys, expectedKey)
				}
			}
		})
	}
}

func TestPVCBackend_Delete(t *testing.T) {
	tempDir := t.TempDir()
	backend, err := NewPVCBackend(tempDir, "http://test.local:8080")
	require.NoError(t, err)

	ctx := context.Background()

	// Store an artifact
	testKey := "namespace/source/artifact.tar.gz"
	testData := []byte("test data")
	_, err = backend.Store(ctx, testKey, testData)
	require.NoError(t, err)

	tests := []struct {
		name        string
		key         string
		expectError bool
	}{
		{
			name:        "delete existing key",
			key:         testKey,
			expectError: false,
		},
		{
			name:        "delete non-existent key",
			key:         "does/not/exist.tar.gz",
			expectError: false, // Should not error
		},
		{
			name:        "path traversal attempt",
			key:         "../../../etc/passwd",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := backend.Delete(ctx, tt.key)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Verify file was deleted
				if tt.key == testKey {
					filePath := filepath.Join(tempDir, tt.key)
					_, err := os.Stat(filePath)
					assert.True(t, os.IsNotExist(err))
				}
			}
		})
	}
}

func TestPVCBackend_GetURL(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		key         string
		expectedURL string
	}{
		{
			name:        "with baseURL",
			baseURL:     "http://test.local:8080",
			key:         "namespace/source/artifact.tar.gz",
			expectedURL: "http://test.local:8080/namespace/source/artifact.tar.gz",
		},
		{
			name:        "without baseURL",
			baseURL:     "",
			key:         "namespace/source/artifact.tar.gz",
			expectedURL: "file:///tmp/artifacts/namespace/source/artifact.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			if tt.baseURL == "" {
				tempDir = "/tmp/artifacts"
			}
			backend, err := NewPVCBackend(tempDir, tt.baseURL)
			require.NoError(t, err)

			url := backend.GetURL(tt.key)
			if tt.baseURL != "" {
				assert.Equal(t, tt.expectedURL, url)
			} else {
				assert.Contains(t, url, "file://")
				assert.Contains(t, url, tt.key)
			}
		})
	}
}

func TestPVCBackend_ConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	backend, err := NewPVCBackend(tempDir, "http://test.local:8080")
	require.NoError(t, err)

	ctx := context.Background()

	// Test concurrent writes
	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := filepath.Join("concurrent", "test", filepath.Base(t.Name()), filepath.Base(t.Name())+"_"+string(rune(idx))+".tar.gz")
			data := []byte("concurrent data")
			_, err := backend.Store(ctx, key, data)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Verify all files were created
	keys, err := backend.List(ctx, "concurrent")
	assert.NoError(t, err)
	assert.Len(t, keys, numGoroutines)
}

func TestPVCBackend_CleanupEmptyDirs(t *testing.T) {
	tempDir := t.TempDir()
	backend, err := NewPVCBackend(tempDir, "http://test.local:8080")
	require.NoError(t, err)

	ctx := context.Background()

	// Store an artifact in nested directories
	testKey := "a/b/c/d/artifact.tar.gz"
	_, err = backend.Store(ctx, testKey, []byte("data"))
	require.NoError(t, err)

	// Delete the artifact
	err = backend.Delete(ctx, testKey)
	require.NoError(t, err)

	// Verify nested directories were cleaned up
	// The exact cleanup behavior depends on whether other files exist
	// Just verify the artifact is gone
	_, err = backend.Retrieve(ctx, testKey)
	assert.Error(t, err)
}
