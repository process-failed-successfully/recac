package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectDevCommand(t *testing.T) {
	// 1. Setup Temp Dir
	tmpDir := t.TempDir()

	// 2. Test Go
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test"), 0644)
	assert.Equal(t, "go test ./...", detectDevCommand(tmpDir))
	os.Remove(filepath.Join(tmpDir, "go.mod"))

	// 3. Test Node
	os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte("{}"), 0644)
	assert.Equal(t, "npm test", detectDevCommand(tmpDir))
	os.Remove(filepath.Join(tmpDir, "package.json"))

	// 4. Test Make
	os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte("all:"), 0644)
	assert.Equal(t, "make", detectDevCommand(tmpDir))
	os.Remove(filepath.Join(tmpDir, "Makefile"))

	// 5. Test Python
	os.WriteFile(filepath.Join(tmpDir, "requirements.txt"), []byte("pytest"), 0644)
	assert.Equal(t, "pytest", detectDevCommand(tmpDir))
	os.Remove(filepath.Join(tmpDir, "requirements.txt"))

	// 6. Test None
	assert.Equal(t, "", detectDevCommand(tmpDir))
}

func TestParseExtensions(t *testing.T) {
	tests := []struct {
		name     string
		flagExt  string
		cmd      string
		expected []string
	}{
		{
			name:     "Explicit Flag",
			flagExt:  ".go,.js",
			cmd:      "go test",
			expected: []string{".go", ".js"},
		},
		{
			name:     "Explicit Flag without dot",
			flagExt:  "py,txt",
			cmd:      "pytest",
			expected: []string{".py", ".txt"},
		},
		{
			name:     "Go Command",
			flagExt:  "",
			cmd:      "go test ./...",
			expected: []string{".go", ".mod"},
		},
		{
			name:     "NPM Command",
			flagExt:  "",
			cmd:      "npm start",
			expected: []string{".js", ".ts", ".json"},
		},
		{
			name:     "Make Command",
			flagExt:  "",
			cmd:      "make",
			expected: []string{".go", ".c", ".cpp", ".h", ".rs"},
		},
		{
			name:     "Python Command",
			flagExt:  "",
			cmd:      "pytest",
			expected: []string{".py"},
		},
		{
			name:     "Unknown Command",
			flagExt:  "",
			cmd:      "echo hello",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseExtensions(tt.flagExt, tt.cmd)
			assert.Equal(t, tt.expected, got)
		})
	}
}
