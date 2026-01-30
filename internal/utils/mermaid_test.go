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
			name:     "Simple ID",
			input:    "SimpleID",
			expected: "SimpleID",
		},
		{
			name:     "ID with dots and slashes",
			input:    "pkg/subpkg.Func",
			expected: "pkg_subpkg_Func",
		},
		{
			name:     "ID with special characters",
			input:    "Func(Arg)*[Index]",
			expected: "Func_Arg___Index_",
		},
		{
			name:     "ID with spaces and quotes",
			input:    `My "Func" 'Name'`,
			expected: `My__Func___Name_`,
		},
		{
			name:     "ID with colon and ampersand",
			input:    "Key:Value&More",
			expected: "Key_Value_More",
		},
		{
			name:     "ID with backticks",
			input:    "`RawString`",
			expected: "_RawString_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeMermaidID(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
