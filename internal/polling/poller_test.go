package polling

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNewConfig(t *testing.T) {
	// Test default configuration
	cfg := NewConfig()
	if cfg.Interval != 5*time.Minute {
		t.Errorf("Expected default interval of 5 minutes, got %v", cfg.Interval)
	}

	// Test environment variable override
	os.Setenv("JIRA_POLLING_INTERVAL", "10m")
	cfg = NewConfig()
	if cfg.Interval != 10*time.Minute {
		t.Errorf("Expected interval of 10 minutes, got %v", cfg.Interval)
	}

	// Test invalid environment variable (should fall back to default)
	os.Setenv("JIRA_POLLING_INTERVAL", "invalid")
	cfg = NewConfig()
	if cfg.Interval != 5*time.Minute {
		t.Errorf("Expected fallback to default interval, got %v", cfg.Interval)
	}

	// Clean up
	os.Unsetenv("JIRA_POLLING_INTERVAL")
}

func TestPollerStart(t *testing.T) {
	cfg := &Config{Interval: 100 * time.Millisecond}
	poller := NewPoller(cfg)

	// Create a context that will be canceled after 350ms
	// This should allow for 3 polling cycles (0ms, 100ms, 200ms, 300ms)
	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()

	// Start the poller in a goroutine
	done := make(chan bool)
	go func() {
		poller.Start(ctx)
		done <- true
	}()

	// Wait for the poller to finish
	select {
	case <-done:
		t.Log("Poller stopped successfully")
	case <-time.After(500 * time.Millisecond):
		t.Error("Poller did not stop within expected time")
	}
}
