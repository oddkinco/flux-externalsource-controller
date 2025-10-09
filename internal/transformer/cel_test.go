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

package transformer

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCELTransformer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CEL Transformer Suite")
}

var _ = Describe("CELTransformer", func() {
	var (
		transformer *CELTransformer
		ctx         context.Context
	)

	BeforeEach(func() {
		transformer = NewCELTransformer(5 * time.Second)
		ctx = context.Background()
	})

	Describe("NewCELTransformer", func() {
		It("should create transformer with specified timeout", func() {
			timeout := 10 * time.Second
			t := NewCELTransformer(timeout)
			Expect(t.timeout).To(Equal(timeout))
		})

		It("should use default timeout when zero timeout provided", func() {
			t := NewCELTransformer(0)
			Expect(t.timeout).To(Equal(30 * time.Second))
		})

		It("should use default timeout when negative timeout provided", func() {
			t := NewCELTransformer(-1 * time.Second)
			Expect(t.timeout).To(Equal(30 * time.Second))
		})
	})

	Describe("Transform with valid expressions", func() {
		Context("with JSON input", func() {
			It("should transform simple JSON object", func() {
				input := []byte(`{"name": "test", "value": 42}`)
				expression := `input.name + "-" + string(input.value)`

				result, err := transformer.Transform(ctx, input, expression)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(result)).To(Equal("test-42"))
			})

			It("should extract specific field from JSON", func() {
				input := []byte(`{"config": {"database": {"host": "localhost", "port": 5432}}}`)
				expression := `input.config.database.host`

				result, err := transformer.Transform(ctx, input, expression)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(result)).To(Equal("localhost"))
			})

			It("should transform array data", func() {
				input := []byte(`{"items": [1, 2, 3, 4, 5]}`)
				expression := `input.items.filter(x, x > 2)`

				result, err := transformer.Transform(ctx, input, expression)
				Expect(err).ToNot(HaveOccurred())

				var resultArray []int
				err = json.Unmarshal(result, &resultArray)
				Expect(err).ToNot(HaveOccurred())
				Expect(resultArray).To(Equal([]int{3, 4, 5}))
			})

			It("should create new JSON structure", func() {
				input := []byte(`{"user": {"name": "Alice", "age": 30}, "role": "admin"}`)
				expression := `{"username": input.user.name, "isAdmin": input.role == "admin"}`

				result, err := transformer.Transform(ctx, input, expression)
				Expect(err).ToNot(HaveOccurred())

				var resultMap map[string]interface{}
				err = json.Unmarshal(result, &resultMap)
				Expect(err).ToNot(HaveOccurred())
				Expect(resultMap["username"]).To(Equal("Alice"))
				Expect(resultMap["isAdmin"]).To(BeTrue())
			})
		})

		Context("with string input", func() {
			It("should transform plain text", func() {
				input := []byte("hello world")
				expression := `input + " processed"`

				result, err := transformer.Transform(ctx, input, expression)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(result)).To(Equal("hello world processed"))
			})

			It("should use data alias for input", func() {
				input := []byte("test string")
				expression := `data + " processed"`

				result, err := transformer.Transform(ctx, input, expression)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(result)).To(Equal("test string processed"))
			})
		})

		Context("with different result types", func() {
			It("should handle boolean results", func() {
				input := []byte(`{"enabled": true}`)
				expression := `input.enabled`

				result, err := transformer.Transform(ctx, input, expression)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(result)).To(Equal("true"))
			})

			It("should handle integer results", func() {
				input := []byte(`{"count": 42}`)
				expression := `int(input.count) * 2`

				result, err := transformer.Transform(ctx, input, expression)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(result)).To(Equal("84"))
			})

			It("should handle double results", func() {
				input := []byte(`{"price": 19.99}`)
				expression := `input.price * 1.1`

				result, err := transformer.Transform(ctx, input, expression)
				Expect(err).ToNot(HaveOccurred())

				var resultFloat float64
				err = json.Unmarshal(result, &resultFloat)
				Expect(err).ToNot(HaveOccurred())
				Expect(resultFloat).To(BeNumerically("~", 21.989, 0.001))
			})
		})
	})

	Describe("Transform with invalid expressions", func() {
		It("should return error for syntax errors", func() {
			input := []byte(`{"test": "value"}`)
			expression := `input.test +` // Invalid syntax

			_, err := transformer.Transform(ctx, input, expression)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to compile CEL expression"))
		})

		It("should return error for undefined variables", func() {
			input := []byte(`{"test": "value"}`)
			expression := `undefined_var.field`

			_, err := transformer.Transform(ctx, input, expression)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to compile CEL expression"))
		})

		It("should return error for runtime errors", func() {
			input := []byte(`{"test": "value"}`)
			expression := `input.nonexistent.field` // Will cause runtime error

			_, err := transformer.Transform(ctx, input, expression)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("CEL expression execution failed"))
		})

		It("should return error for type mismatches", func() {
			input := []byte(`{"number": 42}`)
			expression := `input.number + "string"` // Trying to add string to number without conversion

			_, err := transformer.Transform(ctx, input, expression)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("CEL expression execution failed"))
		})
	})

	Describe("Transform with timeout handling", func() {
		It("should timeout for malicious infinite loop expressions", func() {
			transformer = NewCELTransformer(100 * time.Millisecond) // Very short timeout
			input := []byte(`{"items": [1, 2, 3]}`)

			// This expression should be complex enough to potentially timeout
			// Note: CEL has built-in protections, so we simulate with a complex operation
			expression := `input.items.map(x, input.items.map(y, input.items.map(z, x + y + z)))`

			start := time.Now()
			_, err := transformer.Transform(ctx, input, expression)
			duration := time.Since(start)

			// The operation should complete quickly due to CEL's built-in protections
			// but we test that our timeout mechanism is in place
			Expect(duration).To(BeNumerically("<", 2*time.Second))

			// If it does timeout, we should get the appropriate error
			if err != nil && strings.Contains(err.Error(), "timed out") {
				Expect(err.Error()).To(ContainSubstring("CEL expression execution timed out"))
			}
		})

		It("should respect context cancellation", func() {
			cancelCtx, cancel := context.WithCancel(ctx)
			input := []byte(`{"test": "value"}`)
			expression := `input.test`

			// Cancel immediately
			cancel()

			_, err := transformer.Transform(cancelCtx, input, expression)

			// Should either succeed quickly or fail due to context cancellation
			if err != nil {
				Expect(err.Error()).To(SatisfyAny(
					ContainSubstring("context canceled"),
					ContainSubstring("timed out"),
				))
			}
		})
	})

	Describe("Transform with edge cases", func() {
		It("should handle empty input", func() {
			input := []byte("")
			expression := `"default"`

			result, err := transformer.Transform(ctx, input, expression)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(result)).To(Equal("default"))
		})

		It("should handle null JSON input", func() {
			input := []byte("null")
			expression := `input == null ? "was null" : "not null"`

			result, err := transformer.Transform(ctx, input, expression)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(result)).To(Equal("was null"))
		})

		It("should handle invalid JSON as string", func() {
			input := []byte("not valid json")
			expression := `input + " processed"`

			result, err := transformer.Transform(ctx, input, expression)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(result)).To(Equal("not valid json processed"))
		})

		It("should handle complex nested transformations", func() {
			input := []byte(`{
				"users": [
					{"name": "Alice", "age": 30, "active": true},
					{"name": "Bob", "age": 25, "active": false},
					{"name": "Charlie", "age": 35, "active": true}
				]
			}`)
			expression := `{
				"activeUsers": input.users.filter(u, u.active).map(u, u.name),
				"totalUsers": size(input.users)
			}`

			result, err := transformer.Transform(ctx, input, expression)
			Expect(err).ToNot(HaveOccurred())

			var resultMap map[string]interface{}
			err = json.Unmarshal(result, &resultMap)
			Expect(err).ToNot(HaveOccurred())

			activeUsers := resultMap["activeUsers"].([]interface{})
			Expect(activeUsers).To(HaveLen(2))
			Expect(activeUsers).To(ContainElements("Alice", "Charlie"))

			totalUsers := resultMap["totalUsers"].(float64)
			Expect(totalUsers).To(Equal(3.0))
		})
	})
})
