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
	"log"
	"strings"
	"sync"
)

// MemoryBackend implements StorageBackend for in-memory storage
// WARNING: This backend is non-persistent and data will be lost on controller restart
type MemoryBackend struct {
	data    map[string][]byte
	mutex   sync.RWMutex
	warned  bool
	baseURL string
}

// NewMemoryBackend creates a new in-memory storage backend
// If baseURL is provided, it will be used for generating artifact URLs
// Otherwise, URLs will use the memory:// scheme
func NewMemoryBackend(baseURL ...string) *MemoryBackend {
	backend := &MemoryBackend{
		data: make(map[string][]byte),
	}

	if len(baseURL) > 0 && baseURL[0] != "" {
		backend.baseURL = baseURL[0]
	}

	// Issue warning about non-persistence
	if !backend.warned {
		log.Println("WARNING: Using in-memory storage backend. Artifacts will NOT persist across controller restarts.")
		log.Println("WARNING: This backend is intended for development and testing only.")
		backend.warned = true
	}

	return backend
}

// Store saves data in memory and returns a URL
func (m *MemoryBackend) Store(_ context.Context, key string, data []byte) (string, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Store a copy of the data to prevent external modifications
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	m.data[key] = dataCopy

	// Return URL based on baseURL if set, otherwise use memory:// scheme
	return m.GetURL(key), nil
}

// List returns a list of keys with the given prefix
func (m *MemoryBackend) List(_ context.Context, prefix string) ([]string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var keys []string
	for key := range m.data {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}

	return keys, nil
}

// Delete removes an object from memory
func (m *MemoryBackend) Delete(_ context.Context, key string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.data, key)
	return nil
}

// GetURL returns the URL for accessing the stored object
func (m *MemoryBackend) GetURL(key string) string {
	if m.baseURL != "" {
		return fmt.Sprintf("%s/%s", m.baseURL, key)
	}
	return fmt.Sprintf("memory://localhost/%s", key)
}

// Retrieve retrieves data from memory by key
func (m *MemoryBackend) Retrieve(_ context.Context, key string) ([]byte, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	data, exists := m.data[key]
	if !exists {
		return nil, fmt.Errorf("artifact not found: %s", key)
	}

	// Return a copy to prevent external modifications
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	return dataCopy, nil
}

// GetData returns the stored data for a key (for testing purposes)
// This method is not part of the StorageBackend interface but useful for testing
func (m *MemoryBackend) GetData(key string) ([]byte, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	data, exists := m.data[key]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent external modifications
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	return dataCopy, true
}

// Size returns the number of stored objects (for testing/debugging)
func (m *MemoryBackend) Size() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return len(m.data)
}

// Clear removes all stored objects (for testing purposes)
func (m *MemoryBackend) Clear() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.data = make(map[string][]byte)
}
