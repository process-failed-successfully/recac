package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
