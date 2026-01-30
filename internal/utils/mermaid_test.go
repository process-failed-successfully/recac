package utils

import (
	"testing"
)

func TestSanitizeMermaidID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"pkg.Func", "pkg_Func"},
		{"pkg.(Type).Method", "pkg__Type__Method"},
		{"space bar", "space_bar"},
		{"hyphen-ated", "hyphen_ated"},
		{"slash/path", "slash_path"},
		{"weird chars: &[]", "weird_chars_____"},
	}

	for _, tt := range tests {
		got := SanitizeMermaidID(tt.input)
		if got != tt.expected {
			t.Errorf("SanitizeMermaidID(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}
