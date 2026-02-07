package notify

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/slack-go/slack"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestNewManager_EnvironmentVariables(t *testing.T) {
	// Reset viper
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	// Set env vars
	t.Setenv("SLACK_BOT_USER_TOKEN", "xoxb-test-token")
	t.Setenv("SLACK_APP_TOKEN", "xapp-test-token")
	t.Setenv("DISCORD_BOT_TOKEN", "discord-test-token")
	t.Setenv("DISCORD_CHANNEL_ID", "123456")

	// Set config
	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.discord.enabled", true)
	viper.Set("notifications.slack.channel", "#test")

	m := NewManager(nil)

	assert.NotNil(t, m.client, "Slack client should be initialized")
	assert.NotNil(t, m.socketClient, "Slack socket client should be initialized")
	assert.NotNil(t, m.discordNotifier, "Discord notifier should be initialized")
	assert.Equal(t, "#test", m.channelID)
}

func TestNewManager_SlackDisabled(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	viper.Set("notifications.slack.enabled", false)

	m := NewManager(nil)
	assert.Nil(t, m.client)
}

func TestNewManager_DiscordDisabled(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	viper.Set("notifications.discord.enabled", false)

	m := NewManager(nil)
	assert.Nil(t, m.discordNotifier)
}

func TestManager_Start(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	logged := false
	logger := func(format string, args ...interface{}) {
		logged = true
	}

	m := &Manager{
		logger: logger,
	}

	m.Start(context.Background())
	assert.False(t, logged, "Should not log if socketClient is nil")
}

func TestManager_Notify_Logging(t *testing.T) {
	var logs []string
	logger := func(format string, args ...interface{}) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}

	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.slack.events.on_start", true)

	// Mock failure
	mockSlack := &mockSlackPoster{
		postMessageContextFunc: func(ctx context.Context, channelID string, options ...slack.MsgOption) (string, string, error) {
			return "", "", fmt.Errorf("simulated error")
		},
	}

	m := &Manager{
		logger:    logger,
		client:    mockSlack,
		channelID: "#test",
	}

	m.Notify(context.Background(), EventStart, "test", "")

	// Check if logs contain the failure message
	foundError := false
	for _, l := range logs {
		if strings.Contains(l, "Failed to send Slack notification") {
			foundError = true
			break
		}
	}
	assert.True(t, foundError, "Should log failure")
}

func TestManager_AddReaction_Logging(t *testing.T) {
	var logs []string
	logger := func(format string, args ...interface{}) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}

	mockSlack := &mockSlackPoster{
		addReactionContextFunc: func(ctx context.Context, name string, item slack.ItemRef) error {
			return fmt.Errorf("reaction error")
		},
	}

	m := &Manager{
		logger: logger,
		client: mockSlack,
	}

	m.AddReaction(context.Background(), "ts_1", "smile")

	foundError := false
	for _, l := range logs {
		if strings.Contains(l, "Failed to add Slack reaction") {
			foundError = true
			break
		}
	}
	assert.True(t, foundError, "Should log reaction failure")
}

func TestManager_HandleEvents(t *testing.T) {
	m := &Manager{}
	m.HandleEvents(context.Background())
}

func TestManager_GetStyle_AllCases(t *testing.T) {
	title, color := getStyle(EventSuccess)
	assert.Equal(t, "✅ Success", title)
	assert.Equal(t, "#2eb886", color)

	title, color = getStyle(EventUserInteraction)
	assert.Equal(t, "💬 Input Needed", title)
	assert.Equal(t, "#f1c40f", color)

	title, color = getStyle(EventProjectComplete)
	assert.Equal(t, "🏁 Project Complete", title)
	assert.Equal(t, "#2eb886", color)
}

func TestDiscordNotifier_AddReaction_EmojiMapping_Real(t *testing.T) {
	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	tests := []struct {
		input    string
		expected string
	}{
		{"x", "❌"},
		{":x:", "❌"},
		{"warning", "⚠️"},
		{":warning:", "⚠️"},
		{"custom", "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			notifier := NewDiscordBotNotifier("token", "123")
			// Use testTransport (defined in discord_test.go) to redirect to mock server
			notifier.Client = &http.Client{
				Transport: &testTransport{
					TargetURL: server.URL,
				},
			}

			err := notifier.AddReaction(context.Background(), "msg1", tt.input)
			if err != nil {
				t.Fatalf("AddReaction failed: %v", err)
			}

			// Verify captured path contains emoji (decoded)
			if !strings.Contains(capturedPath, tt.expected) {
				t.Errorf("Path %q should contain %q", capturedPath, tt.expected)
			}
		})
	}
}
