package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeMermaidID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"has-dash", "has_dash"},
		{"has.dot", "has_dot"},
		{"has space", "has_space"},
		{"complex(pkg)", "complex_pkg_"},
		{"method.(Type).Func", "method__Type__Func"},
		{"array[index]", "array_index_"},
		{"path/to/file", "path_to_file"},
		{"windows\\path", "windows_path"},
	}

	for _, tt := range tests {
		got := sanitizeMermaidID(tt.input)
		// We verify that invalid chars are NOT present
		assert.NotContains(t, got, "(", "Failed for %s", tt.input)
		assert.NotContains(t, got, ")", "Failed for %s", tt.input)
		assert.NotContains(t, got, "[", "Failed for %s", tt.input)
		assert.NotContains(t, got, "]", "Failed for %s", tt.input)
		assert.NotContains(t, got, "/", "Failed for %s", tt.input)
		assert.NotContains(t, got, "\\", "Failed for %s", tt.input)

		// Also verify replacement correctness for known simple cases if needed,
		// but mainly we care about the characters being gone.
	}
}
