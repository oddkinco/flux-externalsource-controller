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

package generator

import (
	"context"
)

// SourceGenerator defines the interface for all source generators
type SourceGenerator interface {
	// Generate fetches data from the external source
	Generate(ctx context.Context, config GeneratorConfig) (*SourceData, error)

	// SupportsConditionalFetch returns true if the generator supports conditional fetching
	SupportsConditionalFetch() bool

	// GetLastModified returns the last modified identifier (e.g., ETag) for conditional fetching
	GetLastModified(ctx context.Context, config GeneratorConfig) (string, error)
}

// GeneratorConfig holds configuration for source generators
//
//nolint:revive // Clear naming is more important than avoiding "stuttering"
type GeneratorConfig struct {
	Type   string                 `json:"type"`
	Config map[string]interface{} `json:"config"`
}

// SourceData represents data fetched from an external source
type SourceData struct {
	Data         []byte            `json:"data"`
	LastModified string            `json:"lastModified"`
	Metadata     map[string]string `json:"metadata"`
}

// SourceGeneratorFactory creates source generators based on type
type SourceGeneratorFactory interface {
	// CreateGenerator creates a generator for the specified type
	CreateGenerator(generatorType string) (SourceGenerator, error)

	// RegisterGenerator registers a generator factory function for a type
	RegisterGenerator(generatorType string, factory func() SourceGenerator) error

	// SupportedTypes returns a list of supported generator types
	SupportedTypes() []string
}
