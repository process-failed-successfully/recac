package utils

import (
	"testing"
)

func TestSanitizeMermaidID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Basic ID",
			input:    "funcName",
			expected: "funcName",
		},
		{
			name:     "Dot Separation",
			input:    "pkg.Func",
			expected: "pkg_Func",
		},
		{
			name:     "Parentheses (Methods)",
			input:    "pkg.(Type).Method",
			expected: "pkg__Type__Method",
		},
		{
			name:     "Complex Characters",
			input:    "foo/bar-baz*qux:quux&corge",
			expected: "foo_bar_baz_qux_quux_corge",
		},
		{
			name:     "Quotes and Backticks",
			input:    "name with \"quotes\" and 'single' and `backticks`",
			expected: "name_with__quotes__and__single__and__backticks_",
		},
		{
			name:     "Brackets",
			input:    "[array]index",
			expected: "_array_index",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeMermaidID(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizeMermaidID(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}
