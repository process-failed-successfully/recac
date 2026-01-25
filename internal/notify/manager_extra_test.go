package notify

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/slack-go/slack"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestManager_Init_Warnings(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	// Set enabled but no tokens
	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.discord.enabled", true)

	// Unset env vars
	t.Setenv("SLACK_BOT_USER_TOKEN", "")
	t.Setenv("SLACK_APP_TOKEN", "")
	t.Setenv("DISCORD_BOT_TOKEN", "")
	t.Setenv("DISCORD_CHANNEL_ID", "")

	logs := []string{}
	logger := func(msg string, args ...interface{}) {
		logs = append(logs, msg)
	}

	m := NewManager(logger)
	assert.NotNil(t, m)

	foundSlackWarn := false
	foundDiscordWarn := false

	for _, l := range logs {
		if strings.Contains(l, "SLACK_BOT_USER_TOKEN not set") {
			foundSlackWarn = true
		}
		if strings.Contains(l, "DISCORD_BOT_TOKEN or DISCORD_CHANNEL_ID not set") {
			foundDiscordWarn = true
		}
	}

	assert.True(t, foundSlackWarn, "Should warn about missing Slack token")
	assert.True(t, foundDiscordWarn, "Should warn about missing Discord token")
}

func TestManager_Start_NilSocket(t *testing.T) {
	// Test Start when socketClient is nil (should do nothing/not crash)
	m := &Manager{}
	assert.NotPanics(t, func() {
		m.Start(context.Background())
	})
}

func TestManager_Notify_PartialErrors(t *testing.T) {
	// Test Notify where one provider fails and checks logs
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.slack.events.on_start", true)
	viper.Set("notifications.discord.enabled", true)

	logs := []string{}
	logger := func(msg string, args ...interface{}) {
		// Just verify we get called
		if strings.Contains(msg, "Failed to send") {
			logs = append(logs, msg)
		}
	}

	// Mock Slack Success
	mockSlack := &mockSlackPoster{
		postMessageContextFunc: func(ctx context.Context, channelID string, options ...slack.MsgOption) (string, string, error) {
			return "ch", "ts", nil
		},
	}

	// Mock Discord Failure
	mockDiscord := &mockDiscordPoster{
		sendFunc: func(ctx context.Context, message, threadID string) (string, error) {
			return "", errors.New("discord error")
		},
	}

	m := &Manager{
		client:          mockSlack,
		discordNotifier: mockDiscord,
		logger:          logger,
	}

	ctx := context.Background()
	state, err := m.Notify(ctx, EventStart, "msg", "")
	assert.NoError(t, err) // Should still succeed overall (partial success)

	// State should contain slack TS but not discord ID (since it failed)
	// Output is just "ts" because dumpThreadState returns raw string if only slack is present
	assert.Equal(t, "ts", state)

	// Logs should contain failure
	foundLog := false
	for _, l := range logs {
		if strings.Contains(l, "Failed to send Discord notification") {
			foundLog = true
		}
	}
	assert.True(t, foundLog)
}

func TestManager_AddReaction_PartialErrors(t *testing.T) {
	logs := []string{}
	logger := func(msg string, args ...interface{}) {
		if strings.Contains(msg, "Failed to add") {
			logs = append(logs, msg)
		}
	}

	mockSlack := &mockSlackPoster{
		addReactionContextFunc: func(ctx context.Context, name string, item slack.ItemRef) error {
			return errors.New("slack fail")
		},
	}

	mockDiscord := &mockDiscordPoster{
		addReactionFunc: func(ctx context.Context, messageID, reaction string) error {
			return nil
		},
	}

	m := &Manager{
		client:          mockSlack,
		discordNotifier: mockDiscord,
		logger:          logger,
	}

	tsStr := `{"slack_ts":"ts","discord_id":"id"}`
	err := m.AddReaction(context.Background(), tsStr, "smile")
	assert.NoError(t, err)

	foundLog := false
	for _, l := range logs {
		if strings.Contains(l, "Failed to add Slack reaction") {
			foundLog = true
		}
	}
	assert.True(t, foundLog)
}
