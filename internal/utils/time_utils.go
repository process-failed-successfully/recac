package utils

import (
	"fmt"
	"time"
)

// FormatSince returns a human-readable string representing the time elapsed since t.
func FormatSince(t time.Time) string {
	if t.IsZero() {
		return "never"
	}

	const (
		day  = 24 * time.Hour
		week = 7 * day
	)

	since := time.Since(t)
	if since < time.Minute {
		return fmt.Sprintf("%ds ago", int(since.Seconds()))
	}
	if since < time.Hour {
		return fmt.Sprintf("%dm ago", int(since.Minutes()))
	}
	if since < day {
		return fmt.Sprintf("%dh ago", int(since.Hours()))
	}
	if since < week {
		return fmt.Sprintf("%dd ago", int(since.Hours()/24))
	}
	// Fallback to absolute date for longer durations
	return t.Format("2006-01-02")
}

// ParseDurationWithDays parses a duration string like "7d" or "24h" into a time.Duration.
// It's more flexible than time.ParseDuration by supporting days.
func ParseDurationWithDays(durationStr string) (time.Duration, error) {
	if len(durationStr) < 2 {
		return 0, fmt.Errorf("duration string too short")
	}

	unit := durationStr[len(durationStr)-1]
	valueStr := durationStr[:len(durationStr)-1]
	value, err := time.ParseDuration(valueStr + "h") // Default to hours for parsing
	if err != nil {
		return 0, fmt.Errorf("invalid duration value: %w", err)
	}

	switch unit {
	case 'd':
		return value * 24, nil
	case 'h':
		return value, nil
	default:
		// Fallback to time.ParseDuration for standard units (m, s, etc.)
		return time.ParseDuration(durationStr)
	}
}
