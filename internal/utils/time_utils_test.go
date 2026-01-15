package utils

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseStaleDuration(t *testing.T) {
	tests := []struct {
		name         string
		durationStr  string
		expected     time.Duration
		expectErr    bool
		errContains  string
	}{
		{
			name:        "Valid days",
			durationStr: "7d",
			expected:    7 * 24 * time.Hour,
			expectErr:   false,
		},
		{
			name:        "Valid hours",
			durationStr: "24h",
			expected:    24 * time.Hour,
			expectErr:   false,
		},
		{
			name:        "Valid minutes",
			durationStr: "60m",
			expected:    60 * time.Minute,
			expectErr:   false,
		},
		{
			name:        "Zero value",
			durationStr: "0d",
			expected:    0,
			expectErr:   false,
		},
		{
			name:        "Invalid format - no unit",
			durationStr: "10",
			expectErr:   true,
			errContains: "invalid duration format",
		},
		{
			name:        "Invalid format - unknown unit",
			durationStr: "5w",
			expectErr:   true,
			errContains: "invalid duration format",
		},
		{
			name:        "Invalid format - negative value",
			durationStr: "-7d",
			expectErr:   true,
			errContains: "invalid duration format",
		},
		{
			name:        "Empty string",
			durationStr: "",
			expectErr:   true,
			errContains: "invalid duration format",
		},
		{
			name:        "Valid seconds",
			durationStr: "30s",
			expected:    30 * time.Second,
			expectErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration, err := ParseStaleDuration(tt.durationStr)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, duration)
			}
		})
	}
}

func TestParseTime(t *testing.T) {
	// Freeze time for consistent relative time tests
	now := time.Now()
	tests := []struct {
		name        string
		timeStr     string
		expected    time.Time
		expectErr   bool
		errContains string
	}{
		// Relative time tests
		{
			name:     "Relative time - 7 days",
			timeStr:  "7d",
			expected: now.Add(-7 * 24 * time.Hour),
		},
		{
			name:     "Relative time - 24 hours",
			timeStr:  "24h",
			expected: now.Add(-24 * time.Hour),
		},
		// Absolute time tests
		{
			name:    "Absolute time - valid date",
			timeStr: "2023-10-27",
			expected: time.Date(2023, 10, 27, 0, 0, 0, 0, time.UTC),
		},
		{
			name:    "Absolute time - with whitespace",
			timeStr: "  2023-10-27  ",
			expected: time.Date(2023, 10, 27, 0, 0, 0, 0, time.UTC),
		},
		// Error cases
		{
			name:        "Invalid format - mixed",
			timeStr:     "7days",
			expectErr:   true,
			errContains: "invalid time format",
		},
		{
			name:        "Invalid format - bad date",
			timeStr:     "2023/10/27",
			expectErr:   true,
			errContains: "invalid time format",
		},
		{
			name:        "Empty string",
			timeStr:     "",
			expectErr:   true,
			errContains: "time string cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedTime, err := ParseTime(tt.timeStr)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
				// For relative times, check if it's close enough (within a second)
				if strings.Contains(tt.name, "Relative") {
					assert.WithinDuration(t, tt.expected, parsedTime, time.Second)
				} else { // For absolute times, expect an exact match
					// We need to adjust the location of the parsed time to match the expected UTC.
					assert.Equal(t, tt.expected, parsedTime.In(time.UTC))
				}
			}
		})
	}
}
