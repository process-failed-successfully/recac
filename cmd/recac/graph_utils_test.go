package main

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
			name:     "Simple ID",
			input:    "simple",
			expected: "simple",
		},
		{
			name:     "Spaces and Hyphens",
			input:    "my-id with spaces",
			expected: "my_id_with_spaces",
		},
		{
			name:     "Dots",
			input:    "pkg.Func",
			expected: "pkg_Func",
		},
		{
			name:     "Method with Receiver",
			input:    "pkg.(Type).Method",
			expected: "pkg__Type__Method",
		},
		{
			name:     "Paths",
			input:    "path/to/pkg.Func",
			expected: "path_to_pkg_Func",
		},
		{
			name:     "Pointer Receiver",
			input:    "(*Type).Method",
			expected: "__Type__Method",
		},
		{
			name:     "Ambiguous Node",
			input:    "(Ambiguous).Method",
			expected: "_Ambiguous__Method",
		},
		{
			name:     "Special Characters",
			input:    "weird&id[with]quotes\"'",
			expected: "weird_id_with_quotes__",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeMermaidID(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
