package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeMermaidID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"pkg.Func", "pkg_Func"},
		{"pkg/sub.Func", "pkg_sub_Func"},
		{"pkg.(Type).Method", "pkg__Type__Method"},
		{"with spaces", "with_spaces"},
		{"special-chars: &*()[]", "special_chars________"},
		{"quotes'\"`", "quotes___"},
		{"backslash\\slash", "backslash_slash"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, SanitizeMermaidID(tt.input))
		})
	}
}
