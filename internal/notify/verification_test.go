package notify_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"recac/internal/notify"

	"github.com/spf13/viper"
)

func TestNotificationFlow(t *testing.T) {
	// Setup Config
	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.slack.events.on_start", true)

	// We need to unset the tokens to avoid actually trying to connect in a unit test
	// (unless we mock the client, which is hard with the slack-go library structure).
	// However, the Manager gracefully handles empty tokens or logic.
	// For this test, we are verifying the *config logic* primarily.

	// Save original env
	origBot := os.Getenv("SLACK_BOT_USER_TOKEN")
	os.Setenv("SLACK_BOT_USER_TOKEN", "xoxb-fake")
	defer os.Setenv("SLACK_BOT_USER_TOKEN", origBot)

	// Capture logs to verify intent
	var logs []string
	logger := func(msg string, args ...interface{}) {
		if len(args) > 0 {
			logs = append(logs, fmt.Sprintf(msg, args...))
		} else {
			logs = append(logs, msg)
		}
	}

	mgr := notify.NewManager(logger)
	ctx := context.Background()

	// Test Event Start (Enabled)
	mgr.Notify(ctx, notify.EventStart, "Hello World")

	found := false
	for _, l := range logs {
		if l == "Sending notification for event: on_start" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected log 'Sending notification for event: on_start', got %v", logs)
	}

	// Test Event Disabled
	viper.Set("notifications.slack.events.on_failure", false)
	mgr.Notify(ctx, notify.EventFailure, "Should skip")

	foundFailure := false
	for _, l := range logs {
		if l == "Sending notification for event: on_failure" {
			foundFailure = true
			break
		}
	}
	if foundFailure {
		t.Error("Did not expect notification for disabled event")
	}
}
