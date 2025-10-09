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
	"fmt"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// CELTransformer implements the Transformer interface using CEL (Common Expression Language)
type CELTransformer struct {
	timeout time.Duration
}

// NewCELTransformer creates a new CEL transformer with the specified timeout
func NewCELTransformer(timeout time.Duration) *CELTransformer {
	if timeout <= 0 {
		timeout = 30 * time.Second // Default timeout
	}
	return &CELTransformer{
		timeout: timeout,
	}
}

// Transform applies a CEL expression to the input data
func (c *CELTransformer) Transform(ctx context.Context, input []byte, expression string) ([]byte, error) {
	// Create a context with timeout for sandboxed execution
	timeoutCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Parse input as JSON to make it available to CEL
	var inputData interface{}
	if err := json.Unmarshal(input, &inputData); err != nil {
		// If JSON parsing fails, treat as string
		inputData = string(input)
	}

	// Create CEL environment with input variable and standard functions
	env, err := cel.NewEnv(
		cel.Variable("input", cel.DynType),
		cel.Variable("data", cel.DynType), // Alias for input for convenience
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	// Parse the CEL expression
	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("failed to compile CEL expression: %w", issues.Err())
	}

	// Create program from AST
	program, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL program: %w", err)
	}

	// Execute the program with timeout
	resultChan := make(chan evalResult, 1)
	go func() {
		result, _, err := program.Eval(map[string]interface{}{
			"input": inputData,
			"data":  inputData,
		})
		resultChan <- evalResult{result: result, err: err}
	}()

	select {
	case <-timeoutCtx.Done():
		return nil, fmt.Errorf("CEL expression execution timed out after %v", c.timeout)
	case evalRes := <-resultChan:
		if evalRes.err != nil {
			return nil, fmt.Errorf("CEL expression execution failed: %w", evalRes.err)
		}

		// Convert result to bytes
		return c.convertResultToBytes(evalRes.result)
	}
}

// evalResult holds the result of CEL evaluation
type evalResult struct {
	result ref.Val
	err    error
}

// convertResultToBytes converts a CEL result value to bytes
func (c *CELTransformer) convertResultToBytes(result ref.Val) ([]byte, error) {
	// Handle different CEL result types
	switch result.Type() {
	case types.StringType:
		return []byte(result.Value().(string)), nil
	case types.BytesType:
		return result.Value().([]byte), nil
	case types.BoolType, types.IntType, types.UintType, types.DoubleType:
		// Convert primitive types to JSON
		jsonBytes, err := json.Marshal(result.Value())
		if err != nil {
			return nil, fmt.Errorf("failed to marshal primitive result to JSON: %w", err)
		}
		return jsonBytes, nil
	case types.ListType, types.MapType:
		// Convert CEL types to native Go types first
		nativeValue, err := c.convertCELToNative(result)
		if err != nil {
			return nil, fmt.Errorf("failed to convert CEL result to native type: %w", err)
		}
		jsonBytes, err := json.Marshal(nativeValue)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal complex result to JSON: %w", err)
		}
		return jsonBytes, nil
	default:
		// For unknown types, try to convert to string
		if strVal := result.ConvertToType(types.StringType); strVal.Type() == types.StringType {
			return []byte(strVal.Value().(string)), nil
		}
		return nil, fmt.Errorf("unsupported CEL result type: %v", result.Type())
	}
}

// convertCELToNative recursively converts CEL values to native Go types
func (c *CELTransformer) convertCELToNative(val ref.Val) (interface{}, error) {
	switch val.Type() {
	case types.StringType, types.BytesType, types.BoolType, types.IntType, types.UintType, types.DoubleType:
		return val.Value(), nil
	case types.ListType:
		list := val.Value().([]ref.Val)
		result := make([]interface{}, len(list))
		for i, item := range list {
			converted, err := c.convertCELToNative(item)
			if err != nil {
				return nil, err
			}
			result[i] = converted
		}
		return result, nil
	case types.MapType:
		celMap := val.Value().(map[ref.Val]ref.Val)
		result := make(map[string]interface{})
		for k, v := range celMap {
			keyStr, ok := k.Value().(string)
			if !ok {
				return nil, fmt.Errorf("map key must be string, got %T", k.Value())
			}
			converted, err := c.convertCELToNative(v)
			if err != nil {
				return nil, err
			}
			result[keyStr] = converted
		}
		return result, nil
	default:
		return val.Value(), nil
	}
}