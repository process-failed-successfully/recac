package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectDevCommand(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "recac-dev-detect")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name     string
		file     string
		expected string
	}{
		{"Go", "go.mod", "go test ./..."},
		{"Node", "package.json", "npm test"},
		{"Make", "Makefile", "make"},
		{"Python", "requirements.txt", "pytest"},
		{"None", "other.txt", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create file
			f := filepath.Join(tempDir, tt.file)
			os.WriteFile(f, []byte(""), 0644)
			defer os.Remove(f)

			got := detectDevCommand(tempDir)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestParseExtensions(t *testing.T) {
	tests := []struct {
		name     string
		flag     string
		cmd      string
		expected []string
	}{
		{"Flag", ".js,.ts", "", []string{".js", ".ts"}},
		{"FlagNoDot", "js,ts", "", []string{".js", ".ts"}},
		{"GoCmd", "", "go test", []string{".go", ".mod"}},
		{"NodeCmd", "", "npm test", []string{".js", ".ts", ".json"}},
		{"PythonCmd", "", "pytest", []string{".py"}},
		{"MakeCmd", "", "make build", []string{".go", ".c", ".cpp", ".h", ".rs"}},
		{"Unknown", "", "echo hi", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseExtensions(tt.flag, tt.cmd)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// Note: TestDevCmd (the full integration test) is removed as it relied on
// blocking execution which is now handled by the TUI (Bubble Tea).
// The logic is now covered by unit tests in internal/ui/dev_dashboard_test.go
// and the detection logic tests above.
