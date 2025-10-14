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

// Package hooks provides hook execution capabilities for ExternalSource resources.
package hooks

import (
	"context"

	sourcev1alpha1 "github.com/oddkinco/flux-externalsource-controller/api/v1alpha1"
)

// HookExecutor defines the interface for executing hooks
type HookExecutor interface {
	// Execute runs a hook with the given input data and returns the output
	Execute(ctx context.Context, input []byte, hook sourcev1alpha1.HookSpec) ([]byte, error)
}

// WhitelistManager defines the interface for managing command whitelists
type WhitelistManager interface {
	// IsAllowed checks if a command with given arguments is allowed
	IsAllowed(command string, args []string) bool

	// Reload reloads the whitelist from the configured source
	Reload() error
}
