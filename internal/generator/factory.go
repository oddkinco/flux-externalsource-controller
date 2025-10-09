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
	"fmt"
	"sync"
)

// Factory implements SourceGeneratorFactory
type Factory struct {
	generators map[string]func() SourceGenerator
	mutex      sync.RWMutex
}

// NewFactory creates a new source generator factory
func NewFactory() *Factory {
	return &Factory{
		generators: make(map[string]func() SourceGenerator),
	}
}

// CreateGenerator creates a generator for the specified type
func (f *Factory) CreateGenerator(generatorType string) (SourceGenerator, error) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	factory, exists := f.generators[generatorType]
	if !exists {
		return nil, fmt.Errorf("unsupported generator type: %s", generatorType)
	}

	return factory(), nil
}

// RegisterGenerator registers a generator factory function for a type
func (f *Factory) RegisterGenerator(generatorType string, factory func() SourceGenerator) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if generatorType == "" {
		return fmt.Errorf("generator type cannot be empty")
	}

	if factory == nil {
		return fmt.Errorf("factory function cannot be nil")
	}

	f.generators[generatorType] = factory
	return nil
}

// SupportedTypes returns a list of supported generator types
func (f *Factory) SupportedTypes() []string {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	types := make([]string, 0, len(f.generators))
	for generatorType := range f.generators {
		types = append(types, generatorType)
	}

	return types
}
