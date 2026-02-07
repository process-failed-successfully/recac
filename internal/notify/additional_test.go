package notify

import (
	"context"
	"testing"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// TestInitDiscord tests initialization logic with env vars.
// We cannot easily test NewDiscordBotNotifier logic without mocking http requests inside it,
// but we can test that m.discordNotifier gets set.
func TestInitDiscord(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	t.Setenv("DISCORD_BOT_TOKEN", "test-token")
	t.Setenv("DISCORD_CHANNEL_ID", "123456789")
	viper.Set("notifications.discord.enabled", true)

	m := NewManager(nil)
	assert.NotNil(t, m.discordNotifier)
}

func TestInitDiscord_Disabled(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	t.Setenv("DISCORD_BOT_TOKEN", "test-token")
	viper.Set("notifications.discord.enabled", false)

	m := NewManager(nil)
	assert.Nil(t, m.discordNotifier)
}

func TestStart(t *testing.T) {
	// Start launches a goroutine that calls socketClient.RunContext.
	// We can't easily mock socketClient.RunContext because it's a struct method.
	// But we can check that it doesn't panic and respects context cancellation.

	// Create a dummy socket client
	api := slack.New("xoxb-dummy")
	socketClient := socketmode.New(api)

	m := &Manager{
		socketClient: socketClient,
		logger:       func(string, ...interface{}) {},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Should return immediately (in goroutine) without blocking
	m.Start(ctx)

	// Wait a bit to ensure goroutine started and likely finished
	time.Sleep(10 * time.Millisecond)
}

func TestHandleEvents(t *testing.T) {
	// We want to test that HandleEvents processes events from the channel.
	// We need to inject a socketClient with a channel we control.

	api := slack.New("xoxb-dummy")
	socketClient := socketmode.New(api)
	// Override the channel with our own
	eventsCh := make(chan socketmode.Event)
	socketClient.Events = eventsCh

	mockPoster := &mockSlackPoster{
		postMessageFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
			assert.Equal(t, "C12345", channelID)
			return "", "", nil
		},
	}

	m := &Manager{
		socketClient: socketClient,
		client:       mockPoster,
		logger:       func(string, ...interface{}) {},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start handler in background
	go m.HandleEvents(ctx)

	// 1. Send Connecting event
	eventsCh <- socketmode.Event{Type: socketmode.EventTypeConnecting}

	// 2. Send Connected event
	eventsCh <- socketmode.Event{Type: socketmode.EventTypeConnected}

	// 3. Send EventsAPI event (AppMention)
	// This is complex because we need to construct a valid slackevents.EventsAPIEvent
	innerEvent := slackevents.EventsAPIInnerEvent{
		Type: string(slackevents.AppMention),
		Data: &slackevents.AppMentionEvent{
			User:    "U12345",
			Text:    "Hello bot",
			Channel: "C12345",
		},
	}

	apiEvent := slackevents.EventsAPIEvent{
		Type:       slackevents.CallbackEvent,
		InnerEvent: innerEvent,
	}

	// The handler calls m.socketClient.Ack(*evt.Request).
	// We need to provide a Request to avoid nil pointer dereference if it uses it.
	// socketmode.Event has Request *socketmode.Request
	req := &socketmode.Request{}

	eventsCh <- socketmode.Event{
		Type:    socketmode.EventTypeEventsAPI,
		Data:    apiEvent,
		Request: req,
	}

	// Give it time to process
	time.Sleep(50 * time.Millisecond)

	// Context cancellation stops the loop
	cancel()
	time.Sleep(10 * time.Millisecond)
}
