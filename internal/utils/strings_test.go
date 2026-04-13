package utils

import "testing"

func TestContainsFold(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		substr string
		want   bool
	}{
		{"empty strings", "", "", true},
		{"empty substr", "hello", "", true},
		{"exact match", "hello", "hello", true},
		{"exact match upper", "HELLO", "HELLO", true},
		{"lower s, upper substr", "hello world", "WORLD", true},
		{"upper s, lower substr", "HELLO WORLD", "world", true},
		{"mixed case match", "HeLlO WoRlD", "wOrLd", true},
		{"no match", "hello", "world", false},
		{"substr longer than s", "hi", "hello", false},
		{"partial match at start", "hello world", "he", true},
		{"partial match at end", "hello world", "ld", true},
		{"partial match in middle", "hello world", "lo w", true},
		{"non-ascii", "hellö", "lö", true}, // Simple byte-level match without true fold for non-ascii
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsFold(tt.s, tt.substr); got != tt.want {
				t.Errorf("ContainsFold(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}

func TestHasPrefixFold(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		prefix string
		want   bool
	}{
		{"empty strings", "", "", true},
		{"empty prefix", "hello", "", true},
		{"exact match", "hello", "hello", true},
		{"exact match upper", "HELLO", "HELLO", true},
		{"lower s, upper prefix", "hello world", "HELLO", true},
		{"upper s, lower prefix", "HELLO WORLD", "hello", true},
		{"mixed case match", "HeLlO WoRlD", "hElLo", true},
		{"no match", "hello", "world", false},
		{"prefix longer than s", "hi", "hello", false},
		{"partial match not at start", "hello world", "world", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasPrefixFold(tt.s, tt.prefix); got != tt.want {
				t.Errorf("HasPrefixFold(%q, %q) = %v, want %v", tt.s, tt.prefix, got, tt.want)
			}
		})
	}
}
