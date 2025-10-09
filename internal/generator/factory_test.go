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
	"testing"
)

// mockGenerator implements SourceGenerator for testing
type mockGenerator struct {
	name string
}

func (m *mockGenerator) Generate(ctx context.Context, config GeneratorConfig) (*SourceData, error) {
	return &SourceData{
		Data:         []byte("mock data"),
		LastModified: "mock-etag",
		Metadata:     map[string]string{"generator": m.name},
	}, nil
}

func (m *mockGenerator) SupportsConditionalFetch() bool {
	return true
}

func (m *mockGenerator) GetLastModified(ctx context.Context, config GeneratorConfig) (string, error) {
	return "mock-etag", nil
}

func TestFactory_RegisterGenerator(t *testing.T) {
	factory := NewFactory()

	// Test successful registration
	err := factory.RegisterGenerator("mock", func() SourceGenerator {
		return &mockGenerator{name: "mock"}
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test empty generator type
	err = factory.RegisterGenerator("", func() SourceGenerator {
		return &mockGenerator{}
	})
	if err == nil {
		t.Error("Expected error for empty generator type")
	}

	// Test nil factory function
	err = factory.RegisterGenerator("nil", nil)
	if err == nil {
		t.Error("Expected error for nil factory function")
	}
}

func TestFactory_CreateGenerator(t *testing.T) {
	factory := NewFactory()

	// Register a mock generator
	err := factory.RegisterGenerator("mock", func() SourceGenerator {
		return &mockGenerator{name: "mock"}
	})
	if err != nil {
		t.Fatalf("Failed to register generator: %v", err)
	}

	// Test successful creation
	generator, err := factory.CreateGenerator("mock")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if generator == nil {
		t.Error("Expected generator, got nil")
	}

	// Test unsupported generator type
	_, err = factory.CreateGenerator("unsupported")
	if err == nil {
		t.Error("Expected error for unsupported generator type")
	}
}

func TestFactory_SupportedTypes(t *testing.T) {
	factory := NewFactory()

	// Initially should be empty
	types := factory.SupportedTypes()
	if len(types) != 0 {
		t.Errorf("Expected 0 types, got %d", len(types))
	}

	// Register generators
	factory.RegisterGenerator("mock1", func() SourceGenerator {
		return &mockGenerator{name: "mock1"}
	})
	factory.RegisterGenerator("mock2", func() SourceGenerator {
		return &mockGenerator{name: "mock2"}
	})

	types = factory.SupportedTypes()
	if len(types) != 2 {
		t.Errorf("Expected 2 types, got %d", len(types))
	}

	// Check that both types are present
	typeMap := make(map[string]bool)
	for _, t := range types {
		typeMap[t] = true
	}
	if !typeMap["mock1"] || !typeMap["mock2"] {
		t.Error("Expected both mock1 and mock2 to be supported")
	}
}

func TestFactory_ConcurrentAccess(t *testing.T) {
	factory := NewFactory()

	// Test concurrent registration and creation
	done := make(chan bool, 2)

	// Goroutine 1: Register generators
	go func() {
		for i := 0; i < 10; i++ {
			factory.RegisterGenerator("concurrent1", func() SourceGenerator {
				return &mockGenerator{name: "concurrent1"}
			})
		}
		done <- true
	}()

	// Goroutine 2: Create generators
	go func() {
		factory.RegisterGenerator("concurrent2", func() SourceGenerator {
			return &mockGenerator{name: "concurrent2"}
		})
		for i := 0; i < 10; i++ {
			factory.CreateGenerator("concurrent2")
		}
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	// Verify final state
	types := factory.SupportedTypes()
	if len(types) < 1 {
		t.Error("Expected at least 1 registered type after concurrent access")
	}
}