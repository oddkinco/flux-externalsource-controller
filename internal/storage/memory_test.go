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
	"testing"
)

func TestMemoryBackend_Store(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	tests := []struct {
		name string
		key  string
		data []byte
	}{
		{
			name: "simple store",
			key:  "test-key",
			data: []byte("test data"),
		},
		{
			name: "store with path",
			key:  "artifacts/test.tar.gz",
			data: []byte("archive data"),
		},
		{
			name: "empty data",
			key:  "empty",
			data: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := backend.Store(ctx, tt.key, tt.data)
			if err != nil {
				t.Errorf("Store() error = %v", err)
				return
			}

			expectedURL := "memory://localhost/" + tt.key
			if url != expectedURL {
				t.Errorf("Store() url = %v, want %v", url, expectedURL)
			}

			// Verify data was stored
			storedData, exists := backend.GetData(tt.key)
			if !exists {
				t.Error("data was not stored")
			}

			if string(storedData) != string(tt.data) {
				t.Errorf("stored data = %v, want %v", string(storedData), string(tt.data))
			}
		})
	}
}

func TestMemoryBackend_List(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	// Store test data
	testData := map[string][]byte{
		"artifacts/file1.tar.gz":        []byte("data1"),
		"artifacts/file2.tar.gz":        []byte("data2"),
		"configs/config.json":           []byte("config"),
		"artifacts/nested/file3.tar.gz": []byte("data3"),
	}

	for key, data := range testData {
		_, err := backend.Store(ctx, key, data)
		if err != nil {
			t.Fatalf("failed to store test data: %v", err)
		}
	}

	tests := []struct {
		name     string
		prefix   string
		expected []string
	}{
		{
			name:   "list all artifacts",
			prefix: "artifacts/",
			expected: []string{
				"artifacts/file1.tar.gz",
				"artifacts/file2.tar.gz",
				"artifacts/nested/file3.tar.gz",
			},
		},
		{
			name:   "list configs",
			prefix: "configs/",
			expected: []string{
				"configs/config.json",
			},
		},
		{
			name:     "list non-existent prefix",
			prefix:   "nonexistent/",
			expected: []string{},
		},
		{
			name:   "list all with empty prefix",
			prefix: "",
			expected: []string{
				"artifacts/file1.tar.gz",
				"artifacts/file2.tar.gz",
				"artifacts/nested/file3.tar.gz",
				"configs/config.json",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys, err := backend.List(ctx, tt.prefix)
			if err != nil {
				t.Errorf("List() error = %v", err)
				return
			}

			if len(keys) != len(tt.expected) {
				t.Errorf("List() returned %d keys, expected %d", len(keys), len(tt.expected))
			}

			// Convert to map for easier comparison
			keyMap := make(map[string]bool)
			for _, key := range keys {
				keyMap[key] = true
			}

			for _, expectedKey := range tt.expected {
				if !keyMap[expectedKey] {
					t.Errorf("expected key %s not found in results", expectedKey)
				}
			}
		})
	}
}

func TestMemoryBackend_Delete(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	// Store test data
	key := "test-key"
	data := []byte("test data")
	_, err := backend.Store(ctx, key, data)
	if err != nil {
		t.Fatalf("failed to store test data: %v", err)
	}

	// Verify data exists
	_, exists := backend.GetData(key)
	if !exists {
		t.Fatal("test data was not stored")
	}

	// Delete the data
	err = backend.Delete(ctx, key)
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	// Verify data was deleted
	_, exists = backend.GetData(key)
	if exists {
		t.Error("data still exists after deletion")
	}

	// Delete non-existent key should not error
	err = backend.Delete(ctx, "non-existent")
	if err != nil {
		t.Errorf("Delete() non-existent key error = %v", err)
	}
}

func TestMemoryBackend_GetURL(t *testing.T) {
	backend := NewMemoryBackend()

	tests := []struct {
		name string
		key  string
		want string
	}{
		{
			name: "simple key",
			key:  "test-key",
			want: "memory://localhost/test-key",
		},
		{
			name: "path key",
			key:  "artifacts/test.tar.gz",
			want: "memory://localhost/artifacts/test.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := backend.GetURL(tt.key)
			if got != tt.want {
				t.Errorf("GetURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMemoryBackend_Clear(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	// Store some test data
	testKeys := []string{"key1", "key2", "key3"}
	for _, key := range testKeys {
		_, err := backend.Store(ctx, key, []byte("data"))
		if err != nil {
			t.Fatalf("failed to store test data: %v", err)
		}
	}

	// Verify data exists
	if backend.Size() != 3 {
		t.Errorf("expected 3 items, got %d", backend.Size())
	}

	// Clear all data
	backend.Clear()

	// Verify all data was cleared
	if backend.Size() != 0 {
		t.Errorf("expected 0 items after clear, got %d", backend.Size())
	}

	// Verify individual keys don't exist
	for _, key := range testKeys {
		_, exists := backend.GetData(key)
		if exists {
			t.Errorf("key %s still exists after clear", key)
		}
	}
}
