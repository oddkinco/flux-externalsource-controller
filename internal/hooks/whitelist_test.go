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
	"os"
	"path/filepath"
	"testing"
)

func TestFileWhitelistManager(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name              string
		whitelistContent  string
		command           string
		args              []string
		allowAllByDefault bool
		want              bool
		wantErr           bool
	}{
		{
			name: "allowed command with no argument restrictions",
			whitelistContent: `commands:
  jq:
    allowed: true`,
			command: "jq",
			args:    []string{".field", "-r"},
			want:    true,
		},
		{
			name: "allowed command with matching arguments",
			whitelistContent: `commands:
  jq:
    allowed: true
    argumentPatterns:
      - "^\\..*"
      - "^-[a-z]$"`,
			command: "jq",
			args:    []string{".field", "-r"},
			want:    true,
		},
		{
			name: "allowed command with non-matching arguments",
			whitelistContent: `commands:
  jq:
    allowed: true
    argumentPatterns:
      - "^\\..*"`,
			command: "jq",
			args:    []string{"invalid"},
			want:    false,
		},
		{
			name: "explicitly disallowed command",
			whitelistContent: `commands:
  rm:
    allowed: false`,
			command: "rm",
			args:    []string{"-rf", "/"},
			want:    false,
		},
		{
			name: "command not in whitelist with allow all default",
			whitelistContent: `commands:
  jq:
    allowed: true`,
			command:           "unknown",
			args:              []string{},
			allowAllByDefault: true,
			want:              true,
		},
		{
			name: "command not in whitelist without allow all default",
			whitelistContent: `commands:
  jq:
    allowed: true`,
			command:           "unknown",
			args:              []string{},
			allowAllByDefault: false,
			want:              false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write whitelist file
			whitelistPath := filepath.Join(tmpDir, "whitelist.yaml")
			if err := os.WriteFile(whitelistPath, []byte(tt.whitelistContent), 0644); err != nil {
				t.Fatalf("Failed to write whitelist file: %v", err)
			}

			// Create whitelist manager
			wm, err := NewFileWhitelistManagerWithDefault(whitelistPath, tt.allowAllByDefault)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewFileWhitelistManager() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Test IsAllowed
			got := wm.IsAllowed(tt.command, tt.args)
			if got != tt.want {
				t.Errorf("IsAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFileWhitelistManager_Reload(t *testing.T) {
	tmpDir := t.TempDir()
	whitelistPath := filepath.Join(tmpDir, "whitelist.yaml")

	// Initial whitelist
	initialContent := `commands:
  jq:
    allowed: true`
	if err := os.WriteFile(whitelistPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to write initial whitelist: %v", err)
	}

	wm, err := NewFileWhitelistManager(whitelistPath)
	if err != nil {
		t.Fatalf("Failed to create whitelist manager: %v", err)
	}

	// Test initial state
	if !wm.IsAllowed("jq", []string{}) {
		t.Error("Expected jq to be allowed initially")
	}
	if wm.IsAllowed("yq", []string{}) {
		t.Error("Expected yq to not be allowed initially")
	}

	// Update whitelist
	updatedContent := `commands:
  jq:
    allowed: true
  yq:
    allowed: true`
	if err := os.WriteFile(whitelistPath, []byte(updatedContent), 0644); err != nil {
		t.Fatalf("Failed to update whitelist: %v", err)
	}

	// Reload
	if err := wm.Reload(); err != nil {
		t.Fatalf("Failed to reload whitelist: %v", err)
	}

	// Test updated state
	if !wm.IsAllowed("jq", []string{}) {
		t.Error("Expected jq to still be allowed after reload")
	}
	if !wm.IsAllowed("yq", []string{}) {
		t.Error("Expected yq to be allowed after reload")
	}
}

func TestFileWhitelistManager_InvalidRegex(t *testing.T) {
	tmpDir := t.TempDir()
	whitelistPath := filepath.Join(tmpDir, "whitelist.yaml")

	invalidContent := `commands:
  jq:
    allowed: true
    argumentPatterns:
      - "[[invalid"` // Invalid regex

	if err := os.WriteFile(whitelistPath, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to write whitelist: %v", err)
	}

	_, err := NewFileWhitelistManager(whitelistPath)
	if err == nil {
		t.Error("Expected error for invalid regex pattern")
	}
}
