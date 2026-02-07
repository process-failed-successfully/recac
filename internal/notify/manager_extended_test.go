package notify

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestManager_InitDiscord_Success(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	viper.Set("notifications.discord.enabled", true)
	t.Setenv("DISCORD_BOT_TOKEN", "mock-token")
	t.Setenv("DISCORD_CHANNEL_ID", "mock-channel")

	m := NewManager(func(s string, i ...interface{}) {})

	assert.NotNil(t, m.discordNotifier)
	dn, ok := m.discordNotifier.(*DiscordNotifier)
	if assert.True(t, ok) {
		assert.Equal(t, "mock-token", dn.BotToken)
		assert.Equal(t, "mock-channel", dn.ChannelID)
	}
}

func TestManager_InitDiscord_Disabled(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	viper.Set("notifications.discord.enabled", false)
	t.Setenv("DISCORD_BOT_TOKEN", "mock-token")

	m := NewManager(nil)
	assert.Nil(t, m.discordNotifier)
}

func TestManager_InitDiscord_MissingToken(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	viper.Set("notifications.discord.enabled", true)
	os.Unsetenv("DISCORD_BOT_TOKEN")

	var loggedWarning bool
	logger := func(msg string, args ...interface{}) {
		loggedWarning = true
	}

	m := NewManager(logger)
	assert.Nil(t, m.discordNotifier)
	assert.True(t, loggedWarning, "Expected warning log for missing token")
}

func TestManager_InitSlack_Success(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	viper.Set("notifications.slack.enabled", true)
	t.Setenv("SLACK_BOT_USER_TOKEN", "xoxb-mock-token")
	t.Setenv("SLACK_APP_TOKEN", "xapp-mock-token")
	viper.Set("notifications.slack.channel", "#general")

	m := NewManager(nil)
	assert.NotNil(t, m.client)
	assert.NotNil(t, m.socketClient)
	assert.Equal(t, "#general", m.channelID)
}

func TestManager_InitSlack_NoSocket(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	viper.Set("notifications.slack.enabled", true)
	t.Setenv("SLACK_BOT_USER_TOKEN", "xoxb-mock-token")
	t.Setenv("SLACK_APP_TOKEN", "")

	m := NewManager(nil)
	assert.NotNil(t, m.client)
	assert.Nil(t, m.socketClient)
}

func TestManager_Start_SocketClient(t *testing.T) {
	m := &Manager{
		socketClient: nil,
	}
	m.Start(context.Background())
}

func TestManager_GetStyle_AllCases(t *testing.T) {
	tests := []struct {
		event    string
		expected string
	}{
		{EventStart, "🚀"},
		{EventSuccess, "✅"},
		{EventFailure, "❌"},
		{EventUserInteraction, "💬"},
		{EventProjectComplete, "🏁"},
		{"unknown", "📢"},
	}

	for _, tt := range tests {
		title, _ := getStyle(tt.event)
		assert.Contains(t, title, tt.expected)
	}
}

func TestMapEmoji(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"white_check_mark", "%E2%9C%85"},
		{":white_check_mark:", "%E2%9C%85"},
		{"x", "%E2%9D%8C"},
		{":x:", "%E2%9D%8C"},
		{"warning", "%E2%9A%A0%EF%B8%8F"},
		{":warning:", "%E2%9A%A0%EF%B8%8F"},
		{"other", "other"},
	}

	for _, tt := range tests {
		got := mapEmoji(tt.input)
		assert.Equal(t, tt.expected, got)
	}
}

func TestManager_HandleEvents_NilSocket(t *testing.T) {
	m := &Manager{socketClient: nil}
	// Should return immediately
	m.HandleEvents(context.Background())
}
