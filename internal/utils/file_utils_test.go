package utils

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteLines(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_write.txt")
	lines := []string{"line1", "line2", "line3"}

	err := WriteLines(filePath, lines)
	require.NoError(t, err)

	readBack, err := ReadLines(filePath)
	require.NoError(t, err)
	assert.Equal(t, lines, readBack)

	// Test error: create file in non-existent directory
	invalidPath := filepath.Join(tmpDir, "non_existent_dir", "file.txt")
	err = WriteLines(invalidPath, lines)
	assert.Error(t, err)
}

func TestReadLines_Error(t *testing.T) {
	// Test error: read non-existent file
	_, err := ReadLines("non_existent_file.txt")
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
		assert.Equal(t, tt.expected, IsBinaryExt(tt.ext), "Extension: %s", tt.ext)
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
			assert.Equal(t, tt.expected, IsBinaryContent(tt.content))
		})
	}
}

func TestDefaultIgnoreMap(t *testing.T) {
	m := DefaultIgnoreMap()
	assert.True(t, m[".git"])
	assert.True(t, m["node_modules"])
	assert.True(t, m["TODO.md"])
	assert.False(t, m["main.go"])
}
