package notify

import (
	"testing"
)

func TestMapEmoji(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"white_check_mark", "%E2%9C%85"},
		{":white_check_mark:", "%E2%9C%85"},
		{"x", "%E2%9D%8C"},
		{":x:", "%E2%9D%8C"},
		{"warning", "%E2%9A%A0%EF%B8%8F"},
		{":warning:", "%E2%9A%A0%EF%B8%8F"},
		{"other_emoji", "other_emoji"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapEmoji(tt.input)
			if result != tt.expected {
				t.Errorf("mapEmoji(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}
