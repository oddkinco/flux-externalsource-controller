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

package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"gopkg.in/yaml.v3"
)

// WhitelistConfig represents the whitelist configuration file format
type WhitelistConfig struct {
	// Commands is a map of command names to their configurations
	Commands map[string]CommandConfig `yaml:"commands"`
}

// CommandConfig represents configuration for a single command
type CommandConfig struct {
	// Allowed indicates if the command is allowed (default: true)
	Allowed bool `yaml:"allowed"`

	// ArgumentPatterns are regex patterns that arguments must match (optional)
	// If empty, all arguments are allowed
	ArgumentPatterns []string `yaml:"argumentPatterns,omitempty"`
}

// FileWhitelistManager implements WhitelistManager by loading from a file
type FileWhitelistManager struct {
	path              string
	config            *WhitelistConfig
	argPatterns       map[string][]*regexp.Regexp
	mu                sync.RWMutex
	allowAllByDefault bool
}

// NewFileWhitelistManager creates a new whitelist manager from a file
func NewFileWhitelistManager(path string) (*FileWhitelistManager, error) {
	wm := &FileWhitelistManager{
		path:              path,
		argPatterns:       make(map[string][]*regexp.Regexp),
		allowAllByDefault: false,
	}

	if err := wm.Reload(); err != nil {
		return nil, fmt.Errorf("failed to load whitelist: %w", err)
	}

	return wm, nil
}

// NewFileWhitelistManagerWithDefault creates a whitelist manager with a default allow policy
func NewFileWhitelistManagerWithDefault(path string, allowAllByDefault bool) (*FileWhitelistManager, error) {
	wm := &FileWhitelistManager{
		path:              path,
		argPatterns:       make(map[string][]*regexp.Regexp),
		allowAllByDefault: allowAllByDefault,
	}

	if err := wm.Reload(); err != nil {
		return nil, fmt.Errorf("failed to load whitelist: %w", err)
	}

	return wm, nil
}

// Reload reloads the whitelist from the file
func (w *FileWhitelistManager) Reload() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Read the file
	data, err := os.ReadFile(w.path)
	if err != nil {
		return fmt.Errorf("failed to read whitelist file: %w", err)
	}

	// Parse the YAML
	var config WhitelistConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse whitelist file: %w", err)
	}

	// Compile regex patterns
	argPatterns := make(map[string][]*regexp.Regexp)
	for cmd, cmdConfig := range config.Commands {
		if len(cmdConfig.ArgumentPatterns) > 0 {
			patterns := make([]*regexp.Regexp, 0, len(cmdConfig.ArgumentPatterns))
			for _, pattern := range cmdConfig.ArgumentPatterns {
				re, err := regexp.Compile(pattern)
				if err != nil {
					return fmt.Errorf("invalid regex pattern for command %s: %w", cmd, err)
				}
				patterns = append(patterns, re)
			}
			argPatterns[cmd] = patterns
		}
	}

	w.config = &config
	w.argPatterns = argPatterns

	return nil
}

// IsAllowed checks if a command with given arguments is allowed
func (w *FileWhitelistManager) IsAllowed(command string, args []string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// If no configuration loaded, use default policy
	if w.config == nil || w.config.Commands == nil {
		return w.allowAllByDefault
	}

	// Check if command is in whitelist
	cmdConfig, exists := w.config.Commands[command]
	if !exists {
		// Check if we should allow based on basename
		basename := filepath.Base(command)
		cmdConfig, exists = w.config.Commands[basename]
		if !exists {
			return w.allowAllByDefault
		}
	}

	// Check if command is explicitly allowed
	if !cmdConfig.Allowed {
		return false
	}

	// If no argument patterns specified, allow all arguments
	patterns, hasPatterns := w.argPatterns[command]
	if !hasPatterns {
		// Try basename
		patterns, hasPatterns = w.argPatterns[filepath.Base(command)]
		if !hasPatterns {
			return true
		}
	}

	// Check each argument against all patterns
	// All arguments must match at least one pattern
	for _, arg := range args {
		matched := false
		for _, pattern := range patterns {
			if pattern.MatchString(arg) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}
