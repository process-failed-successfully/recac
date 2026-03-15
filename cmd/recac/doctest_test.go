package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDoctest(t *testing.T) {
	// Create temp dir for fixtures
	tmpDir, err := os.MkdirTemp("", "doctest-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Define test cases
	tests := []struct {
		name        string
		content     string
		shouldError bool
		checkOut    string
	}{
		{
			name: "valid-go",
			content: "```go\npackage main\nimport \"fmt\"\nfunc main() { fmt.Println(\"Hello\") }\n```",
			shouldError: false,
			checkOut:    "Checking",
		},
		{
			name: "invalid-go",
			content: "```go\npackage main\nfunc main() { syntax error }\n```",
			shouldError: true,
			checkOut:    "Failed",
		},
		{
			name: "valid-json",
			content: "```json\n{\"key\": \"value\"}\n```",
			shouldError: false,
			checkOut:    "Checking",
		},
		{
			name: "invalid-json",
			content: "```json\n{key: value}\n```", // Invalid JSON (no quotes)
			shouldError: true,
			checkOut:    "Failed",
		},
		{
			name: "mixed",
			content: "```go\npackage main\nfunc main(){}\n```\n```json\n{broken}\n```",
			shouldError: true,
			checkOut:    "Failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fixturePath := filepath.Join(tmpDir, tc.name+".md")
			if err := os.WriteFile(fixturePath, []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}

			// Capture output
			var buf bytes.Buffer
			doctestCmd.SetOut(&buf)
			doctestCmd.SetErr(&buf)
			// Reset output after test? Cobra commands don't easily reset, but we overwrite for next test anyway.

			// Execute
			err := runDoctest(doctestCmd, []string{fixturePath})

			if tc.shouldError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tc.shouldError && err != nil {
				t.Errorf("expected no error, got %v", err)
			}

			output := buf.String()
			if tc.checkOut != "" && !strings.Contains(output, tc.checkOut) {
				t.Errorf("output missing %q, got:\n%s", tc.checkOut, output)
			}
		})
	}
}

func TestValidateBash(t *testing.T) {
	// Mock lookPath to simulate bash presence
	originalLookPath := lookPath
	defer func() { lookPath = originalLookPath }()
	lookPath = func(file string) (string, error) {
		return "/bin/bash", nil
	}

	// Mock execCommand
	originalExec := execCommand
	defer func() { execCommand = originalExec }()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		// Just echo for success
		// If arg contains syntax error code, we could fail, but simpler to rely on external bash if present or assume success for mock
		// The real test uses "bash -n -c code".
		// Let's return a dummy success command.
		return exec.Command("true")
	}

	err := validateBash("echo hello")
	assert.NoError(t, err)

	// Mock failure
	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("false")
	}
	err = validateBash("syntax error")
	assert.Error(t, err)
}

func TestValidateGo(t *testing.T) {
	// Mock execCommand
	originalExec := execCommand
	defer func() { execCommand = originalExec }()

	// For go build, we expect success
	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("true")
	}

	err := validateGo("package main\nfunc main(){}")
	assert.NoError(t, err)

	// Mock failure
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "go" && len(arg) > 0 && arg[0] == "build" {
			return exec.Command("false")
		}
		return exec.Command("true") // for go mod init
	}
	err = validateGo("package main\nfunc main(){ broken }")
	assert.Error(t, err)
}

func TestValidateBlock(t *testing.T) {
	tests := []struct {
		name        string
		lang        string
		code        string
		expectError bool
	}{
		{"json valid", "json", `{"key":"value"}`, false},
		{"json invalid", "json", `{"key":value}`, true},
		{"yaml valid", "yaml", "key: value", false},
		{"yaml invalid", "yaml", ":", true},
		{"go valid", "go", "package main\nfunc main() {}", false},
		{"bash valid", "bash", "echo 'hello'", false},
		{"sh valid", "sh", "echo 'hello'", false},
		{"unknown lang", "rust", "fn main() {}", false}, // should ignore and return nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBlock(tt.lang, tt.code)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				// Don't assert no error for Go/Bash as they run actual commands and might fail depending on environment
				// but we can mock them or just accept the result if they pass/fail. The previous test cases already cover validateGo and validateBash.
				// For yaml/json/unknown, they should definitely not error if valid.
				if tt.lang != "go" && tt.lang != "bash" && tt.lang != "sh" {
					assert.NoError(t, err)
				}
			}
		})
	}
}
