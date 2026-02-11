package notify

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestNewManager_WithEnvVars(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.discord.enabled", true)
	viper.Set("notifications.slack.channel", "#test-channel")
	viper.Set("notifications.discord.channel", "123456789")

	// Set Env Vars
	os.Setenv("SLACK_BOT_USER_TOKEN", "xoxb-test-token")
	os.Setenv("SLACK_APP_TOKEN", "xapp-test-token")
	os.Setenv("DISCORD_BOT_TOKEN", "discord-test-token")
	os.Setenv("DISCORD_CHANNEL_ID", "987654321")

	defer func() {
		os.Unsetenv("SLACK_BOT_USER_TOKEN")
		os.Unsetenv("SLACK_APP_TOKEN")
		os.Unsetenv("DISCORD_BOT_TOKEN")
		os.Unsetenv("DISCORD_CHANNEL_ID")
	}()

	m := NewManager(nil)
	assert.NotNil(t, m)
	assert.NotNil(t, m.client, "Slack client should be initialized")
	assert.NotNil(t, m.socketClient, "Slack socket client should be initialized")
	assert.NotNil(t, m.discordNotifier, "Discord notifier should be initialized")

	// Since we can't easily inspect unexported fields without reflection or looking at behavior,
	// checking they are not nil is good enough for NewManager coverage.
}

func TestManager_Start(t *testing.T) {
	// Tests that Start doesn't panic even if socketClient is nil or configured
	m := &Manager{}
	m.Start(context.Background()) // Should be no-op

	// Now with a mock socket client?
	// socketmode.Client is a struct, not interface. Hard to mock directly without interface.
	// But we can check coverage of the nil check.
}

func TestManager_Init_Disabled(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	viper.Set("notifications.slack.enabled", false)
	viper.Set("notifications.discord.enabled", false)

	m := NewManager(nil)
	assert.Nil(t, m.client)
	assert.Nil(t, m.discordNotifier)
}

func TestManager_Init_MissingTokens(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.discord.enabled", true)

	// Ensure env vars are unset
	os.Unsetenv("SLACK_BOT_USER_TOKEN")
	os.Unsetenv("DISCORD_BOT_TOKEN")

	m := NewManager(func(msg string, args ...interface{}) {}) // Capture logs
	assert.Nil(t, m.client)
	// Discord might fallback or be nil depending on implementation.
	// Manager logic:
	// if botToken != "" && channelID != "" { ... } else { log warning }
	assert.Nil(t, m.discordNotifier)
}
