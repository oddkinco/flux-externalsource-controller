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

package artifact

import (
	"context"
)

// ArtifactManager defines the interface for artifact packaging and management
type ArtifactManager interface {
	// Package creates an artifact from the given data and path
	Package(ctx context.Context, data []byte, path string) (*Artifact, error)

	// Store uploads the artifact to the storage backend and returns the URL
	Store(ctx context.Context, artifact *Artifact) (string, error)

	// Cleanup removes obsolete artifacts, keeping only the specified revision
	Cleanup(ctx context.Context, source string, keepRevision string) error
}

// Artifact represents a packaged artifact
type Artifact struct {
	Data     []byte            `json:"data"`
	Path     string            `json:"path"`
	Revision string            `json:"revision"`
	Metadata map[string]string `json:"metadata"`
}
