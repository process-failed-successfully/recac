package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Supported layout for absolute time parsing
const AbsoluteTimeLayout = "2006-01-02"

// StaleDurationRegex matches patterns like "7d", "24h", "60m", "30s"
var StaleDurationRegex = regexp.MustCompile(`^(\d+)([hmsd])$`)

// ParseStaleDuration parses a string like "7d" into a time.Duration.
// It supports days (d), hours (h), and minutes (m).
func ParseStaleDuration(durationStr string) (time.Duration, error) {
	matches := StaleDurationRegex.FindStringSubmatch(durationStr)
	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid duration format: %q. Expected format like '7d', '24h', '60m'", durationStr)
	}

	value, _ := strconv.Atoi(matches[1])
	unit := matches[2]

	switch unit {
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "h":
		return time.Duration(value) * time.Hour, nil
	case "m":
		return time.Duration(value) * time.Minute, nil
	case "s":
		return time.Duration(value) * time.Second, nil
	default:
		// This case should not be reached due to the regex, but is here for safety
		return 0, fmt.Errorf("unsupported time unit: %s", unit)
	}
}

// ParseTime parses a string that can be either a relative duration (e.g., "7d")
// or an absolute timestamp in "YYYY-MM-DD" format. It returns the calculated time.
func ParseTime(timeStr string) (time.Time, error) {
	timeStr = strings.TrimSpace(timeStr)
	if timeStr == "" {
		return time.Time{}, fmt.Errorf("time string cannot be empty")
	}

	// Try parsing as a relative duration first
	if duration, err := ParseStaleDuration(timeStr); err == nil {
		return time.Now().Add(-duration), nil
	}

	// If it fails, try parsing as an absolute timestamp
	if t, err := time.Parse(AbsoluteTimeLayout, timeStr); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("invalid time format: %q. Use '7d'/'24h' or 'YYYY-MM-DD'", timeStr)
}
