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
)

// ArtifactManager defines the interface for artifact packaging and management
type ArtifactManager interface {
	// Package creates an artifact from the given data and path
	Package(ctx context.Context, data []byte, path string) (*Artifact, error)

	// Store uploads the artifact to the storage backend and returns the URL
	Store(ctx context.Context, artifact *Artifact, source string) (string, error)

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
