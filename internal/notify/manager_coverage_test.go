package notify

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestManager_InitSlack_Coverage(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	// 1. Disabled
	viper.Set("notifications.slack.enabled", false)
	m := NewManager(nil)
	assert.Nil(t, m.client)

	// 2. Enabled but no token
	viper.Reset()
	viper.Set("notifications.slack.enabled", true)
	// t.Setenv restores env after test. We need to ensure token is unset.
	t.Setenv("SLACK_BOT_USER_TOKEN", "")

	logCalled := false
	m = NewManager(func(msg string, args ...interface{}) {
		if msg == "Warning: SLACK_BOT_USER_TOKEN not set, slack notifications disabled" {
			logCalled = true
		}
	})
	assert.Nil(t, m.client)
	assert.True(t, logCalled)

	// 3. Enabled, token present, no socket
	t.Setenv("SLACK_BOT_USER_TOKEN", "xoxb-test")
	t.Setenv("SLACK_APP_TOKEN", "")
	m = NewManager(nil)
	assert.NotNil(t, m.client)
	assert.Nil(t, m.socketClient)

	// 4. Enabled, token present, socket enabled (xapp-)
	t.Setenv("SLACK_APP_TOKEN", "xapp-test")
	m = NewManager(nil)
	assert.NotNil(t, m.client)
	assert.NotNil(t, m.socketClient)
}

func TestManager_InitDiscord_Coverage(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	// 1. Disabled
	viper.Set("notifications.discord.enabled", false)
	m := NewManager(nil)
	assert.Nil(t, m.discordNotifier)

	// 2. Enabled but no tokens
	viper.Reset()
	viper.Set("notifications.discord.enabled", true)
	t.Setenv("DISCORD_BOT_TOKEN", "")
	t.Setenv("DISCORD_CHANNEL_ID", "")
	viper.Set("notifications.discord.channel", "")

	logCalled := false
	m = NewManager(func(msg string, args ...interface{}) {
		if msg == "Warning: DISCORD_BOT_TOKEN or DISCORD_CHANNEL_ID not set, discord notifications disabled" {
			logCalled = true
		}
	})
	assert.Nil(t, m.discordNotifier)
	assert.True(t, logCalled)

	// 3. Enabled, Bot Token Present, No Channel
	t.Setenv("DISCORD_BOT_TOKEN", "bot-token")
	m = NewManager(nil)
	assert.Nil(t, m.discordNotifier)

	// 4. Enabled, Bot Token Present, Channel Configured via Viper
	viper.Set("notifications.discord.channel", "123456")
	m = NewManager(nil)
	assert.NotNil(t, m.discordNotifier)
	dn, ok := m.discordNotifier.(*DiscordNotifier)
	assert.True(t, ok)
	assert.Equal(t, "123456", dn.ChannelID)
	assert.Equal(t, "bot-token", dn.BotToken)

	// 5. Enabled, Bot Token Present, Channel Configured via Env
	t.Setenv("DISCORD_CHANNEL_ID", "999999")
	m = NewManager(nil)
	assert.NotNil(t, m.discordNotifier)
	dn, ok = m.discordNotifier.(*DiscordNotifier)
	assert.True(t, ok)
	assert.Equal(t, "999999", dn.ChannelID)
}

func TestManager_Start(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	// Test Start with nil socket client (should do nothing/not panic)
	m := &Manager{}
	m.Start(context.Background())

	// Test Start with socket client
	t.Setenv("SLACK_BOT_USER_TOKEN", "xoxb-test")
	t.Setenv("SLACK_APP_TOKEN", "xapp-test")
	viper.Set("notifications.slack.enabled", true)

	logChan := make(chan string, 10)
	m = NewManager(func(msg string, args ...interface{}) {
		logChan <- msg
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m.Start(ctx)

	// We expect "Starting Slack Socket Mode..."
	select {
	case msg := <-logChan:
		assert.Equal(t, "Starting Slack Socket Mode...", msg)
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for start log")
	}
}
