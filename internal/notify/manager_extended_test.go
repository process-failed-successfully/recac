package notify

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestNewManager_InitLogic(t *testing.T) {
	// Reset viper
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	// Case 1: Disabled via config
	viper.Set("notifications.slack.enabled", false)
	viper.Set("notifications.discord.enabled", false)

	m := NewManager(nil)
	assert.Nil(t, m.client)
	assert.Nil(t, m.discordNotifier)

	// Case 2: Enabled but missing env vars
	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.discord.enabled", true)
	// Ensure env vars are unset
	t.Setenv("SLACK_BOT_USER_TOKEN", "")
	t.Setenv("DISCORD_BOT_TOKEN", "")
	t.Setenv("DISCORD_CHANNEL_ID", "")

	m = NewManager(nil)
	assert.Nil(t, m.client)
	// Discord falls back to webhook check, if webhook not set, it might be nil or created with empty?
	// initDiscord: if botToken != "" && channelID != "" -> NewDiscordBotNotifier
	// else -> logger warning. m.discordNotifier remains nil.
	assert.Nil(t, m.discordNotifier)

	// Case 3: Enabled and env vars present
	t.Setenv("SLACK_BOT_USER_TOKEN", "xoxb-token")
	t.Setenv("SLACK_APP_TOKEN", "xapp-token")
	t.Setenv("DISCORD_BOT_TOKEN", "discord-token")
	t.Setenv("DISCORD_CHANNEL_ID", "123456")

	m = NewManager(nil)
	assert.NotNil(t, m.client)
	assert.NotNil(t, m.discordNotifier)
}

func TestStart(t *testing.T) {
	// Start runs a goroutine for socket mode.
	// We want to verify it doesn't crash.
	// We can't easily verify it runs without mocking socketClient, which is hard created in initSlack.
	// But initSlack only creates socketClient if appToken starts with "xapp-".

	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	viper.Set("notifications.slack.enabled", true)
	t.Setenv("SLACK_BOT_USER_TOKEN", "xoxb-token")
	t.Setenv("SLACK_APP_TOKEN", "xapp-token")

	m := NewManager(func(msg string, args ...interface{}) {})

	// Start it
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m.Start(ctx)

	// Give it a moment (it might try to connect and fail, but shouldn't panic)
	time.Sleep(100 * time.Millisecond)
}
