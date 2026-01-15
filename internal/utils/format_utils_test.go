package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatSince(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		t        time.Time
		expected string
	}{
		{"Zero time", time.Time{}, "N/A"},
		{"Seconds ago", now.Add(-30 * time.Second), "30s ago"},
		{"Minutes ago", now.Add(-15 * time.Minute), "15m ago"},
		{"Hours ago", now.Add(-5 * time.Hour), "5h ago"},
		{"Days ago", now.Add(-3 * 24 * time.Hour), "3d ago"},
		{"Weeks ago", now.Add(-2 * 7 * 24 * time.Hour), "2w ago"},
		{"Months ago", now.Add(-4 * 30 * 24 * time.Hour), "4mo ago"},
		{"Years ago", now.Add(-2 * 365 * 24 * time.Hour), "2y ago"},
		{"Future time", now.Add(1 * time.Hour), "0s ago"}, // Should be handled gracefully
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, FormatSince(tt.t))
		})
	}
}
