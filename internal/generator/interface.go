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
