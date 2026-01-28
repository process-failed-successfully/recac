package main

import (
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

func TestWriteLines(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_write.txt")
	lines := []string{"line1", "line2", "line3"}

	err := writeLines(filePath, lines)
	require.NoError(t, err)

	readBack, err := readLines(filePath)
	require.NoError(t, err)
	assert.Equal(t, lines, readBack)

	// Test error: create file in non-existent directory
	invalidPath := filepath.Join(tmpDir, "non_existent_dir", "file.txt")
	err = writeLines(invalidPath, lines)
	assert.Error(t, err)
}

func TestReadLines_Error(t *testing.T) {
	// Test error: read non-existent file
	_, err := readLines("non_existent_file.txt")
	assert.Error(t, err)
}

func TestIsBinaryExt(t *testing.T) {
	tests := []struct {
		ext      string
		expected bool
	}{
		{".exe", true},
		{".jpg", true},
		{".go", false},
		{".txt", false},
		{".PDF", false}, // Case sensitive in switch?
		{".pdf", true},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, isBinaryExt(tt.ext), "Extension: %s", tt.ext)
	}
}

func TestIsBinaryContent(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		{"Empty", []byte{}, false},
		{"Text", []byte("hello world"), false},
		{"Binary", []byte{0x00, 0x01, 0x02}, true},
		{"Mixed", []byte("hello\x00world"), true},
		{"LongText", []byte(strings.Repeat("a", 1000)), false},
		{"LongBinaryDetection", append([]byte(strings.Repeat("a", 500)), 0x00), true},
		{"LongBinaryBeyondLimit", append([]byte(strings.Repeat("a", 600)), 0x00), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isBinaryContent(tt.content))
		})
	}
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

func TestSanitizeMermaidID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"pkg.Func", "pkg_Func"},
		{"pkg-name.Func", "pkg_name_Func"},
		{"pkg.(Type).Method", "pkg__Type__Method"}, // Expecting parens to be replaced
		{"path/to/pkg.Func", "path_to_pkg_Func"}, // Expecting slashes to be replaced
		{"[ambiguous].Func", "_ambiguous__Func"}, // Expecting brackets to be replaced
		{"*StarExpr", "_StarExpr"}, // Expecting asterisks to be replaced
		{"user@example.com", "user_example_com"}, // Special characters
		{"a+b=c", "a_b_c"}, // Math operators
		{"100%", "100_"}, // Percent
		{"param,list", "param_list"}, // Comma
		{"'quoted'", "_quoted_"}, // Quotes
		{`"double"`, "_double_"}, // Double quotes
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := sanitizeMermaidID(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}
