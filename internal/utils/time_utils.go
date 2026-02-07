package utils

import (
	"fmt"
	"strconv"
	"strings"
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

// ParseStaleDuration parses a duration string that may contain days (d).
// It supports 'd' suffix for days (24h) and standard time.ParseDuration formats.
// Example: "2d", "1.5d", "48h", "30m"
func ParseStaleDuration(durationStr string) (time.Duration, error) {
	if len(durationStr) < 2 {
		return 0, fmt.Errorf("duration string too short")
	}

	// Handle 'd' (days) suffix
	if strings.HasSuffix(durationStr, "d") {
		valStr := strings.TrimSuffix(durationStr, "d")
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid day value: %w", err)
		}
		return time.Duration(val * float64(24*time.Hour)), nil
	}

	// Fallback to standard time.ParseDuration
	return time.ParseDuration(durationStr)
}
