package notify

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestManager_GetStyle_AllEvents(t *testing.T) {
	tests := []struct {
		event         string
		expectedTitle string
		expectedColor string
	}{
		{EventStart, "🚀 Project Started", "#3498db"},
		{EventSuccess, "✅ Success", "#2eb886"},
		{EventFailure, "❌ Failure", "#a30200"},
		{EventUserInteraction, "💬 Input Needed", "#f1c40f"},
		{EventProjectComplete, "🏁 Project Complete", "#2eb886"},
		{"unknown", "📢 Notification", "#808080"},
	}

	for _, tt := range tests {
		title, color := getStyle(tt.event)
		assert.Equal(t, tt.expectedTitle, title)
		assert.Equal(t, tt.expectedColor, color)
	}
}

func TestDiscord_MapEmoji(t *testing.T) {
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
		result := mapEmoji(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestManager_InitDiscord_Env(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	viper.Set("notifications.discord.enabled", true)
	t.Setenv("DISCORD_BOT_TOKEN", "test-token")
	t.Setenv("DISCORD_CHANNEL_ID", "test-channel")

	m := NewManager(nil)
	assert.NotNil(t, m.discordNotifier)

	dn, ok := m.discordNotifier.(*DiscordNotifier)
	assert.True(t, ok)
	assert.Equal(t, "test-token", dn.BotToken)
	assert.Equal(t, "test-channel", dn.ChannelID)
}

func TestManager_InitDiscord_Fallback(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	viper.Set("notifications.discord.enabled", true)
	t.Setenv("DISCORD_BOT_TOKEN", "test-token")
	// Channel ID from viper
	viper.Set("notifications.discord.channel", "viper-channel")

	m := NewManager(nil)
	assert.NotNil(t, m.discordNotifier)

	dn, ok := m.discordNotifier.(*DiscordNotifier)
	assert.True(t, ok)
	assert.Equal(t, "test-token", dn.BotToken)
	assert.Equal(t, "viper-channel", dn.ChannelID)
}

func TestManager_InitDiscord_Disabled(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	viper.Set("notifications.discord.enabled", false)
	t.Setenv("DISCORD_BOT_TOKEN", "test-token")
	t.Setenv("DISCORD_CHANNEL_ID", "test-channel")

	m := NewManager(nil)
	assert.Nil(t, m.discordNotifier)
}

func TestManager_InitSlack_SocketMode(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	viper.Set("notifications.slack.enabled", true)
	t.Setenv("SLACK_BOT_USER_TOKEN", "xoxb-test")
	t.Setenv("SLACK_APP_TOKEN", "xapp-test")

	m := NewManager(nil)
	assert.NotNil(t, m.client)
	assert.NotNil(t, m.socketClient)
}

func TestManager_InitSlack_NoToken(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	viper.Set("notifications.slack.enabled", true)
	// No env vars
	t.Setenv("SLACK_BOT_USER_TOKEN", "")

	loggerCalled := false
	logger := func(msg string, args ...interface{}) {
		if strings.Contains(msg, "SLACK_BOT_USER_TOKEN not set") {
			loggerCalled = true
		}
	}

	m := NewManager(logger)
	assert.Nil(t, m.client)
	assert.True(t, loggerCalled)
}

func TestManager_Start(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	m := &Manager{}
	m.Start(context.Background())
}

func TestManager_HandleEvents(t *testing.T) {
	// Create a dummy socket client
	api := slack.New("xoxb-test")
	sc := socketmode.New(api)

	// Create manager
	m := &Manager{
		socketClient: sc,
		client: &mockSlackPoster{},
		logger: func(msg string, args ...interface{}) {},
	}

	// Create context with cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run HandleEvents in goroutine
	go m.HandleEvents(ctx)

	// Send events
	sc.Events <- socketmode.Event{
		Type: socketmode.EventTypeConnecting,
	}

	time.Sleep(10 * time.Millisecond)

	sc.Events <- socketmode.Event{
		Type: socketmode.EventTypeConnected,
	}

	time.Sleep(10 * time.Millisecond)

	sc.Events <- socketmode.Event{
		Type: socketmode.EventTypeConnectionError,
	}

	time.Sleep(10 * time.Millisecond)
}
