package polling

import (
	"os"
	"strconv"
	"time"
)

// Config holds the polling configuration
type Config struct {
	Interval time.Duration
}

// NewConfig creates a new polling configuration from environment variables
func NewConfig() *Config {
	// Default interval is 5 minutes
	defaultInterval := 5 * time.Minute

	// Check for environment variable override
	intervalStr := os.Getenv("JIRA_POLLING_INTERVAL")
	if intervalStr != "" {
		// Parse the interval (expecting format like "5m", "1h", "30s")
		interval, err := time.ParseDuration(intervalStr)
		if err == nil {
			return &Config{Interval: interval}
		}
	}

	return &Config{Interval: defaultInterval}
}
