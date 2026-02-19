package notify

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestNewManager_InitSlack(t *testing.T) {
	// Setup
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	os.Setenv("SLACK_BOT_USER_TOKEN", "xoxb-test-token")
	os.Setenv("SLACK_APP_TOKEN", "xapp-test-token")
	t.Cleanup(func() {
		os.Unsetenv("SLACK_BOT_USER_TOKEN")
		os.Unsetenv("SLACK_APP_TOKEN")
	})

	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.slack.channel", "#test")

	// Act
	m := NewManager(nil)

	// Assert
	assert.NotNil(t, m.client)
	assert.Equal(t, "#test", m.channelID)
	assert.NotNil(t, m.socketClient)
}

func TestNewManager_InitDiscord(t *testing.T) {
	// Setup
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	os.Setenv("DISCORD_BOT_TOKEN", "discord-test-token")
	os.Setenv("DISCORD_CHANNEL_ID", "123456")
	t.Cleanup(func() {
		os.Unsetenv("DISCORD_BOT_TOKEN")
		os.Unsetenv("DISCORD_CHANNEL_ID")
	})

	viper.Set("notifications.discord.enabled", true)

	// Act
	m := NewManager(nil)

	// Assert
	assert.NotNil(t, m.discordNotifier)
}

func TestManager_Start(t *testing.T) {
	// Setup
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	os.Setenv("SLACK_BOT_USER_TOKEN", "xoxb-test-token")
	os.Setenv("SLACK_APP_TOKEN", "xapp-test-token")
	t.Cleanup(func() {
		os.Unsetenv("SLACK_BOT_USER_TOKEN")
		os.Unsetenv("SLACK_APP_TOKEN")
	})

	viper.Set("notifications.slack.enabled", true)

	// We can't really mock socketmode.Client deeply without changing the code significantly,
	// but we can ensure Start() doesn't panic.
	// However, socketmode.Client.Run() blocks.
	// Manager.Start() runs it in a goroutine.

	m := NewManager(func(msg string, args ...interface{}) {}) // No-op logger

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Act
	m.Start(ctx)

	// Wait a bit to ensure goroutine started and didn't panic immediately
	time.Sleep(100 * time.Millisecond)

	// Verify? We can verify m.socketClient is not nil
	assert.NotNil(t, m.socketClient)
}
