package notify

import (
	"context"
	"testing"
	"github.com/spf13/viper"
)

func TestManager_InitSlack(t *testing.T) {
	t.Setenv("SLACK_BOT_USER_TOKEN", "xoxb-token")
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	viper.Set("notifications.slack.enabled", true)

	m := NewManager(nil)
	if m.client == nil {
		t.Error("Expected slack client to be initialized")
	}
}

func TestManager_InitDiscord_Bot(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "TOKEN")
	t.Setenv("DISCORD_CHANNEL_ID", "123")
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	viper.Set("notifications.discord.enabled", true)

	m := NewManager(nil)
	if m.discordNotifier == nil {
		t.Error("Expected discord notifier to be initialized")
	}
}

func TestManager_InitDiscord_NoToken(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "")
	t.Setenv("DISCORD_CHANNEL_ID", "")

	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	viper.Set("notifications.discord.enabled", true)

	m := NewManager(nil)
	if m.discordNotifier != nil {
		t.Error("Expected discord notifier to be nil")
	}
}

func TestManager_Start_SocketMode(t *testing.T) {
	t.Setenv("SLACK_BOT_USER_TOKEN", "xoxb-fake")
	t.Setenv("SLACK_APP_TOKEN", "xapp-fake")
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	viper.Set("notifications.slack.enabled", true)

	m := NewManager(nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	m.Start(ctx)
}

func TestManager_GetStyle_Extra(t *testing.T) {
	title, _ := getStyle(EventSuccess)
	if title != "✅ Success" { t.Error("Expected Success title") }

	title, _ = getStyle(EventUserInteraction)
	if title != "💬 Input Needed" { t.Error("Expected Input Needed title") }

	title, _ = getStyle(EventProjectComplete)
	if title != "🏁 Project Complete" { t.Error("Expected Project Complete title") }
}
