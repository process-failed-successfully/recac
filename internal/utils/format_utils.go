package utils

import (
	"fmt"
	"time"
)

// FormatSince returns a human-readable string representing the time elapsed
// since the given timestamp.
func FormatSince(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}

	const (
		day   = 24 * time.Hour
		week  = 7 * day
		month = 30 * day
		year  = 365 * day
	)

	since := time.Since(t)

	// Handle future timestamps gracefully
	if since < 0 {
		return "0s ago"
	}

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
	if since < month {
		return fmt.Sprintf("%dw ago", int(since.Hours()/(24*7)))
	}
	if since < year {
		return fmt.Sprintf("%dmo ago", int(since.Hours()/(24*30)))
	}

	return fmt.Sprintf("%dy ago", int(since.Hours()/(24*365)))
}
