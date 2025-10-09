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
