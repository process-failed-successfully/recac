package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultIgnoreMap(t *testing.T) {
	m := DefaultIgnoreMap()
	assert.True(t, m[".git"])
	assert.True(t, m["node_modules"])
	assert.True(t, m["TODO.md"])
	assert.False(t, m["main.go"])
}

func TestExtractFileContexts(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a dummy file
	fileName := "testfile.go"
	filePath := filepath.Join(tmpDir, fileName)
	content := "package main\nfunc main() {}"
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)

	// Change to temp dir so relative paths work
	t.Chdir(tmpDir)

	tests := []struct {
		name   string
		output string
		check  func(*testing.T, string, error)
	}{
		{
			name:   "No match",
			output: "Error in something",
			check: func(t *testing.T, s string, err error) {
				assert.NoError(t, err)
				assert.Contains(t, s, "No specific files identified")
			},
		},
		{
			name:   "Match existing file",
			output: "Error in testfile.go:1:1",
			check: func(t *testing.T, s string, err error) {
				assert.NoError(t, err)
				assert.Contains(t, s, "File: testfile.go")
				assert.Contains(t, s, content)
			},
		},
		{
			name:   "Match non-existent file",
			output: "Error in missing.go:1",
			check: func(t *testing.T, s string, err error) {
				assert.NoError(t, err)
				assert.Contains(t, s, "Files referenced in output could not be read")
			},
		},
		{
			name:   "Truncated file",
			output: "Error in large.txt:1",
			check: func(t *testing.T, s string, err error) {
				// Create large file
				largeContent := strings.Repeat("a", 11*1024)
				os.WriteFile("large.txt", []byte(largeContent), 0644)

				res, err := extractFileContexts("large.txt:1")
				assert.NoError(t, err)
				assert.Contains(t, res, "... (truncated)")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := extractFileContexts(tt.output)
			tt.check(t, s, err)
		})
	}
}

func TestExtractFileContexts_ReadError(t *testing.T) {
	// Setup mock
	originalReadFileFunc := readFileFunc
	defer func() { readFileFunc = originalReadFileFunc }()

	tmpDir := t.TempDir()
	fileName := "readable.go"
	filePath := filepath.Join(tmpDir, fileName)
	// Create file so os.Stat passes
	os.WriteFile(filePath, []byte("content"), 0644)

	// Change to temp dir so relative paths work
	t.Chdir(tmpDir)

	readFileFunc = func(name string) ([]byte, error) {
		return nil, os.ErrPermission
	}

	output := fmt.Sprintf("Error in %s:1", fileName)
	result, err := extractFileContexts(output)

	assert.NoError(t, err)
	assert.Contains(t, result, fmt.Sprintf("Could not read file %s", fileName))
}
