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
		{"with space", "with_space"},
		{"pkg.Func", "pkg_Func"},
		{"pkg/sub.Func", "pkg_sub_Func"},
		{"(Type).Method", "_Type__Method"},
		{"illegal-chars:[]&*", "illegal_chars_____"},
		{"quotes\"'`", "quotes___"},
		{"back\\slash", "back_slash"},
		{"unhandled_chars;{}|", "unhandled_chars____"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SanitizeMermaidID(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizeMermaidID(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
