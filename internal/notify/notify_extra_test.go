package notify

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockLogger to capture logs
type MockLogger struct {
	Logs []string
}

func (m *MockLogger) Log(format string, args ...interface{}) {
	m.Logs = append(m.Logs, format)
}

func TestInitDiscord(t *testing.T) {
	// Setup Viper
	viper.Set("notifications.discord.enabled", true)
	defer viper.Set("notifications.discord.enabled", false)
	viper.Set("notifications.slack.enabled", false)

	t.Run("Missing Token", func(t *testing.T) {
		os.Unsetenv("DISCORD_BOT_TOKEN")
		os.Unsetenv("DISCORD_CHANNEL_ID")

		mockLog := &MockLogger{}
		m := NewManager(mockLog.Log)

		// initDiscord should have run and logged warning
		found := false
		for _, l := range mockLog.Logs {
			if l == "Warning: DISCORD_BOT_TOKEN or DISCORD_CHANNEL_ID not set, discord notifications disabled" {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected warning log")
		assert.Nil(t, m.discordNotifier)
	})

	t.Run("With Token", func(t *testing.T) {
		os.Setenv("DISCORD_BOT_TOKEN", "dummy-token")
		os.Setenv("DISCORD_CHANNEL_ID", "123")
		defer os.Unsetenv("DISCORD_BOT_TOKEN")
		defer os.Unsetenv("DISCORD_CHANNEL_ID")

		m := NewManager(nil)
		assert.NotNil(t, m.discordNotifier)
	})
}

// MockSlackPoster
type MockSlackPoster struct {
	mock.Mock
}

func (m *MockSlackPoster) PostMessageContext(ctx context.Context, channelID string, options ...slack.MsgOption) (string, string, error) {
	args := m.Called(ctx, channelID, options)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *MockSlackPoster) PostMessage(channelID string, options ...slack.MsgOption) (string, string, error) {
	args := m.Called(channelID, options)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *MockSlackPoster) AddReactionContext(ctx context.Context, name string, item slack.ItemRef) error {
	args := m.Called(ctx, name, item)
	return args.Error(0)
}

func TestHandleEvents(t *testing.T) {
	// Create a real socket client to inject events
	api := slack.New("dummy-token", slack.OptionAppLevelToken("xapp-dummy"))
	sc := socketmode.New(api)

	mockPoster := new(MockSlackPoster)

	m := &Manager{
		socketClient: sc,
		client:       mockPoster,
		logger:       func(string, ...interface{}) {},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Handler in goroutine
	go m.HandleEvents(ctx)

	// Simulate AppMentionEvent
	// We need to construct the event exactly as socketmode expects
	// socketmode.Client.Events is a channel of socketmode.Event

	// Create the inner event payload
	innerEvent := slackevents.EventsAPIEvent{
		Type: slackevents.CallbackEvent,
		InnerEvent: slackevents.EventsAPIInnerEvent{
			Type: "app_mention", // string literal matches slackevents
			Data: &slackevents.AppMentionEvent{
				Type:    "app_mention",
				User:    "U12345",
				Text:    "Hello Bot",
				Channel: "C12345",
			},
		},
	}

	// Create the socketmode event
	evt := socketmode.Event{
		Type: socketmode.EventTypeEventsAPI,
		Data: innerEvent,
		Request: &socketmode.Request{EnvelopeID: "env-1"}, // Needed for Ack
	}

	// Expect PostMessage call
	done := make(chan struct{})
	mockPoster.On("PostMessage", "C12345", mock.Anything).Return("", "", nil).Run(func(args mock.Arguments) {
		close(done)
	}).Once()

	// Inject event
	sc.Events <- evt

	// Wait for completion or timeout
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for PostMessage call")
	}

	mockPoster.AssertExpectations(t)
}

func TestStart(t *testing.T) {
	// Test Start with nil socketClient (safe no-op)
	m := &Manager{socketClient: nil}
	m.Start(context.Background())

	// Test Start with socketClient
	// Since RunContext blocks, we can't easily test it without it running forever or failing.
	// But Start runs it in a goroutine.
	// So calling Start should return immediately.

	api := slack.New("dummy")
	sc := socketmode.New(api)
	m2 := &Manager{socketClient: sc, logger: func(s string, args ...interface{}){}}

	// We just want to ensure it doesn't panic
	m2.Start(context.Background())
}
