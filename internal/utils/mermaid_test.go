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
		{"pkg-Func", "pkg_Func"},
		{"pkg/Func", "pkg_Func"},
		{"pkg\\Func", "pkg_Func"},
		{"(Type).Method", "_Type__Method"},
		{"[index]", "_index_"},
		{"key:val", "key_val"},
		{"A & B", "A___B"},
		{"\"quoted\"", "_quoted_"},
		{"'quoted'", "_quoted_"},
		{"`quoted`", "_quoted_"},
		{"Complex*ID", "Complex_ID"},
	}

	for _, tt := range tests {
		got := SanitizeMermaidID(tt.input)
		if got != tt.expected {
			t.Errorf("SanitizeMermaidID(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
