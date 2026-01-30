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
		{"abc", "abc"},
		{"abc def", "abc_def"},
		{"abc-def", "abc_def"},
		{"abc.def", "abc_def"},
		{"abc(def)", "abc_def_"},
		{"pkg.(Type).Method", "pkg__Type__Method"},
		{"123_456", "123_456"},
		{"!@#$%", "_____"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, SanitizeMermaidID(tt.input))
		})
	}
}
