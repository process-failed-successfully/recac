package notify

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestManager_Init_Full(t *testing.T) {
	// Setup env
	os.Setenv("SLACK_BOT_USER_TOKEN", "xoxb-test")
	os.Setenv("SLACK_APP_TOKEN", "xapp-test")
	os.Setenv("DISCORD_BOT_TOKEN", "discord-token")
	os.Setenv("DISCORD_CHANNEL_ID", "123")

	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.discord.enabled", true)
	viper.Set("notifications.slack.channel", "#general")

	defer func() {
		os.Unsetenv("SLACK_BOT_USER_TOKEN")
		os.Unsetenv("SLACK_APP_TOKEN")
		os.Unsetenv("DISCORD_BOT_TOKEN")
		os.Unsetenv("DISCORD_CHANNEL_ID")
		viper.Reset()
	}()

	m := NewManager(func(s string, i ...interface{}) {})

	assert.NotNil(t, m.client) // Slack client
	assert.NotNil(t, m.socketClient) // Socket client (because xapp-)
	assert.NotNil(t, m.discordNotifier)

	// Test Start with socket client
	// It starts a goroutine. We can't verify execution easily but we can cover the line.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	// Allow goroutine to start
	time.Sleep(10 * time.Millisecond)
}

func TestManager_Init_MissingTokens(t *testing.T) {
	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.discord.enabled", true)

	// No tokens set
	os.Unsetenv("SLACK_BOT_USER_TOKEN")
	os.Unsetenv("DISCORD_BOT_TOKEN")

	m := NewManager(nil)

	assert.Nil(t, m.client)
	assert.Nil(t, m.discordNotifier)
}
