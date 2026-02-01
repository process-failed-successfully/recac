package notify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// Mocks

type mockSlackPoster struct {
	postMessageContextFunc func(ctx context.Context, channelID string, options ...slack.MsgOption) (string, string, error)
	postMessageFunc        func(channelID string, options ...slack.MsgOption) (string, string, error)
	addReactionContextFunc func(ctx context.Context, name string, item slack.ItemRef) error
}

func (m *mockSlackPoster) PostMessageContext(ctx context.Context, channelID string, options ...slack.MsgOption) (string, string, error) {
	if m.postMessageContextFunc != nil {
		return m.postMessageContextFunc(ctx, channelID, options...)
	}
	return "", "", nil
}

func (m *mockSlackPoster) PostMessage(channelID string, options ...slack.MsgOption) (string, string, error) {
	if m.postMessageFunc != nil {
		return m.postMessageFunc(channelID, options...)
	}
	return "", "", nil
}

func (m *mockSlackPoster) AddReactionContext(ctx context.Context, name string, item slack.ItemRef) error {
	if m.addReactionContextFunc != nil {
		return m.addReactionContextFunc(ctx, name, item)
	}
	return nil
}

type mockDiscordPoster struct {
	sendFunc        func(ctx context.Context, message, threadID string) (string, error)
	addReactionFunc func(ctx context.Context, messageID, reaction string) error
}

func (m *mockDiscordPoster) Send(ctx context.Context, message, threadID string) (string, error) {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, message, threadID)
	}
	return "", nil
}

func (m *mockDiscordPoster) AddReaction(ctx context.Context, messageID, reaction string) error {
	if m.addReactionFunc != nil {
		return m.addReactionFunc(ctx, messageID, reaction)
	}
	return nil
}

// Tests

func TestManager_Config(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.discord.enabled", false)
	viper.Set("notifications.slack.channel", "#test-channel")

	m := NewManager(nil)
	assert.NotNil(t, m)

	assert.True(t, m.isProviderEnabled("slack"))
	assert.False(t, m.isProviderEnabled("discord"))
}

func TestManager_IsEnabled(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.slack.events.on_start", true)
	viper.Set("notifications.slack.events.on_failure", false)

	m := NewManager(nil)

	assert.True(t, m.isEnabled(EventStart))
	assert.False(t, m.isEnabled(EventFailure))
	assert.False(t, m.isEnabled(EventSuccess))
}

func TestManager_ThreadState(t *testing.T) {
	jsonState := `{"slack_ts":"123.456","discord_id":"789"}`
	ts := parseThreadState(jsonState)
	assert.Equal(t, "123.456", ts.SlackTS)
	assert.Equal(t, "789", ts.DiscordID)

	legacyState := "123.456"
	tsLegacy := parseThreadState(legacyState)
	assert.Equal(t, "123.456", tsLegacy.SlackTS)
	assert.Empty(t, tsLegacy.DiscordID)

	emptyState := ""
	tsEmpty := parseThreadState(emptyState)
	assert.Empty(t, tsEmpty.SlackTS)

	tsOut := ThreadState{SlackTS: "111", DiscordID: "222"}
	out := dumpThreadState(tsOut)
	assert.Contains(t, out, `"slack_ts":"111"`)
	assert.Contains(t, out, `"discord_id":"222"`)

	tsSlackOnly := ThreadState{SlackTS: "111"}
	outSlack := dumpThreadState(tsSlackOnly)
	assert.Equal(t, "111", outSlack)
}

func TestManager_Notify_Disabled(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	viper.Set("notifications.slack.enabled", false)
	viper.Set("notifications.discord.enabled", false)

	m := NewManager(nil)
	ctx := context.Background()

	state, err := m.Notify(ctx, EventStart, "test message", "")
	assert.NoError(t, err)
	assert.Empty(t, state)
}

func TestManager_GetStyle(t *testing.T) {
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

func TestManager_Notify_Success(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.slack.events.on_start", true)
	viper.Set("notifications.discord.enabled", true)

	slackCalled := false
	discordCalled := false

	mockSlack := &mockSlackPoster{
		postMessageContextFunc: func(ctx context.Context, channelID string, options ...slack.MsgOption) (string, string, error) {
			slackCalled = true
			return "channel", "slack_ts_1", nil
		},
	}
	mockDiscord := &mockDiscordPoster{
		sendFunc: func(ctx context.Context, message, threadID string) (string, error) {
			discordCalled = true
			return "discord_id_1", nil
		},
	}

	m := &Manager{
		client:          mockSlack,
		discordNotifier: mockDiscord,
		channelID:       "#test",
	}

	ctx := context.Background()
	state, err := m.Notify(ctx, EventStart, "message", "")
	assert.NoError(t, err)
	assert.Contains(t, state, `"slack_ts":"slack_ts_1"`)
	assert.Contains(t, state, `"discord_id":"discord_id_1"`)
	assert.True(t, slackCalled)
	assert.True(t, discordCalled)
}

func TestManager_Notify_Failure(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.slack.events.on_start", true)
	viper.Set("notifications.discord.enabled", true)

	mockSlack := &mockSlackPoster{
		postMessageContextFunc: func(ctx context.Context, channelID string, options ...slack.MsgOption) (string, string, error) {
			return "", "", errors.New("slack error")
		},
	}
	mockDiscord := &mockDiscordPoster{
		sendFunc: func(ctx context.Context, message, threadID string) (string, error) {
			return "", errors.New("discord error")
		},
	}

	m := &Manager{
		client:          mockSlack,
		discordNotifier: mockDiscord,
		channelID:       "#test",
		logger:          func(fmt string, args ...interface{}) {}, // absorb logs
	}

	ctx := context.Background()
	state, err := m.Notify(ctx, EventStart, "message", "")

	assert.NoError(t, err)
	assert.Equal(t, "{}", state)
}

func TestManager_AddReaction(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	slackCalled := false
	discordCalled := false

	mockSlack := &mockSlackPoster{
		addReactionContextFunc: func(ctx context.Context, name string, item slack.ItemRef) error {
			slackCalled = true
			assert.Equal(t, "thumbsup", name)
			assert.Equal(t, "ts_1", item.Timestamp)
			return nil
		},
	}

	mockDiscord := &mockDiscordPoster{
		addReactionFunc: func(ctx context.Context, messageID, reaction string) error {
			discordCalled = true
			assert.Equal(t, "did_1", messageID)
			assert.Equal(t, "thumbsup", reaction)
			return nil
		},
	}

	m := &Manager{
		client:          mockSlack,
		discordNotifier: mockDiscord,
		channelID:       "#test",
	}

	threadState := `{"slack_ts":"ts_1","discord_id":"did_1"}`
	err := m.AddReaction(context.Background(), threadState, "thumbsup")
	assert.NoError(t, err)
	assert.True(t, slackCalled)
	assert.True(t, discordCalled)
}

func TestManager_InitDiscord_Env(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	viper.Set("notifications.discord.enabled", true)

	t.Setenv("DISCORD_BOT_TOKEN", "bot_token")
	t.Setenv("DISCORD_CHANNEL_ID", "channel_id")

	m := NewManager(nil)
	assert.NotNil(t, m.discordNotifier)
}

func TestManager_InitSlack_Socket(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	viper.Set("notifications.slack.enabled", true)

	t.Setenv("SLACK_BOT_USER_TOKEN", "bot_token")
	t.Setenv("SLACK_APP_TOKEN", "xapp-test")

	m := NewManager(nil)
	assert.NotNil(t, m.client)
	assert.NotNil(t, m.socketClient)
}

func TestManager_Start_Socket(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	viper.Set("notifications.slack.enabled", true)
	t.Setenv("SLACK_BOT_USER_TOKEN", "bot_token")
	t.Setenv("SLACK_APP_TOKEN", "xapp-test")

	m := NewManager(nil)
	assert.NotNil(t, m.socketClient)

	// Just start and cancel quickly
	ctx, cancel := context.WithCancel(context.Background())
	m.Start(ctx)
	cancel()
	// No panic means success
}

func TestManager_HandleEvents(t *testing.T) {
	// Create a client (mocked api if needed, but not used for this event)
	api := slack.New("xoxo-dummy")
	client := socketmode.New(api)

	logBuffer := bytes.Buffer{}
	logger := func(format string, args ...interface{}) {
		fmt.Fprintf(&logBuffer, format+"\n", args...)
	}

	m := &Manager{
		socketClient: client,
		logger:       logger,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run HandleEvents in background
	go m.HandleEvents(ctx)

	// Send events
	client.Events <- socketmode.Event{Type: socketmode.EventTypeConnecting}
	client.Events <- socketmode.Event{Type: socketmode.EventTypeConnectionError}
	client.Events <- socketmode.Event{Type: socketmode.EventTypeConnected}

	// Use Eventually instead of Sleep
	assert.Eventually(t, func() bool {
		logs := logBuffer.String()
		return strings.Contains(logs, "Connecting to Slack Socket Mode") &&
			strings.Contains(logs, "Connection failed. Retrying later") &&
			strings.Contains(logs, "Connected to Slack Socket Mode")
	}, 2*time.Second, 50*time.Millisecond, "Expected logs to contain socket events")
}
