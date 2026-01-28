package utils

import (
	"testing"
	"time"
)

func TestFormatSince(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		t        time.Time
		expected string
	}{
		{"Zero time", time.Time{}, "never"},
		{"Seconds ago", now.Add(-10 * time.Second), "10s ago"},
		{"Minutes ago", now.Add(-10 * time.Minute), "10m ago"},
		{"Hours ago", now.Add(-10 * time.Hour), "10h ago"},
		{"Days ago", now.Add(-3 * 24 * time.Hour), "3d ago"},
		{"Long ago", now.Add(-30 * 24 * time.Hour), now.Add(-30 * 24 * time.Hour).Format("2006-01-02")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSince(tt.t)
			if got != tt.expected {
				t.Errorf("FormatSince(%v) = %v; want %v", tt.t, got, tt.expected)
			}
		})
	}
}

func TestParseDurationWithDays(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"10s", 10 * time.Second, false},
		{"10m", 10 * time.Minute, false},
		{"10h", 10 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseDurationWithDays(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDurationWithDays(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseDurationWithDays(%q) = %v; want %v", tt.input, got, tt.expected)
			}
		})
	}
}
