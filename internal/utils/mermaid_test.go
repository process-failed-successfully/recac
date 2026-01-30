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
		// New cases
		{"pkg+name", "pkg_name"},
		{"email@example.com", "email_example_com"},
		{"comma,sep", "comma_sep"},
		{"hash#tag", "hash_tag"},
		{"pipe|line", "pipe_line"},
		{"less<than", "less_than"},
		{"greater>than", "greater_than"},
		{"equals=val", "equals_val"},
		{"percent%20", "percent_20"},
		{"dollar$bill", "dollar_bill"},
		{"exclamation!", "exclamation_"},
		{"question?", "question_"},
		{"curly{brace}", "curly_brace_"},
		{"tilde~wave", "tilde_wave"},
		{"caret^up", "caret_up"},
	}

	for _, tt := range tests {
		got := SanitizeMermaidID(tt.input)
		if got != tt.expected {
			t.Errorf("SanitizeMermaidID(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
