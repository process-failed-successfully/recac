package utils

import (
	"testing"
)

func TestSanitizeMermaidID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"pkg.Func", "pkg_Func"},
		{"pkg.(Service).DoWork", "pkg__Service__DoWork"},
		{"pkg-func", "pkg_func"},
		{"pkg func", "pkg_func"},
		{"simple", "simple"},
		{"Mixed_Case_123", "Mixed_Case_123"},
	}

	for _, tt := range tests {
		result := SanitizeMermaidID(tt.input)
		if result != tt.expected {
			t.Errorf("SanitizeMermaidID(%q) = %q; want %q", tt.input, result, tt.expected)
		}
	}
}
