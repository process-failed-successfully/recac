package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeMermaidID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple alphanumeric",
			input:    "Node1",
			expected: "Node1",
		},
		{
			name:     "with spaces",
			input:    "Node 1",
			expected: "Node_1",
		},
		{
			name:     "with dashes and dots",
			input:    "Node-1.2",
			expected: "Node_1_2",
		},
		{
			name:     "with parentheses",
			input:    "func(arg)",
			expected: "func_arg_",
		},
		{
			name:     "complex signature",
			input:    "pkg.(Type).Method",
			expected: "pkg__Type__Method",
		},
		{
			name:     "with slashes",
			input:    "path/to/file",
			expected: "path_to_file",
		},
		{
			name:     "special chars",
			input:    "foo@bar#baz!",
			expected: "foo_bar_baz_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeMermaidID(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
