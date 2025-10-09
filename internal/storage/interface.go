/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package storage

import (
	"context"
)

// StorageBackend defines the interface for artifact storage backends
type StorageBackend interface {
	// Store uploads data to the storage backend and returns the URL
	Store(ctx context.Context, key string, data []byte) (string, error)

	// List returns a list of keys with the given prefix
	List(ctx context.Context, prefix string) ([]string, error)

	// Delete removes an object from storage
	Delete(ctx context.Context, key string) error

	// GetURL returns the URL for accessing the stored object
	GetURL(key string) string
}
