package notify

import (
	"context"
	"errors"
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
		event    string
		expected string
		color    string
	}{
		{EventStart, "🚀 Project Started", "#3498db"},
		{EventSuccess, "✅ Success", "#2eb886"},
		{EventFailure, "❌ Failure", "#a30200"},
		{EventUserInteraction, "💬 Input Needed", "#f1c40f"},
		{EventProjectComplete, "🏁 Project Complete", "#2eb886"},
		{"unknown_event", "📢 Notification", "#808080"},
	}

	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			title, color := getStyle(tt.event)
			assert.Equal(t, tt.expected, title)
			assert.Equal(t, tt.color, color)
		})
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

func TestManager_Init_WithEnv(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	t.Setenv("SLACK_BOT_USER_TOKEN", "xoxb-test")
	t.Setenv("SLACK_APP_TOKEN", "xapp-test")
	t.Setenv("DISCORD_BOT_TOKEN", "discord-test")
	t.Setenv("DISCORD_CHANNEL_ID", "123456")

	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.discord.enabled", true)
	viper.Set("notifications.slack.channel", "#test")

	m := NewManager(nil)
	assert.NotNil(t, m)
	assert.NotNil(t, m.client)
	assert.NotNil(t, m.socketClient) // xapp- prefix triggers socket client
	assert.NotNil(t, m.discordNotifier)
}

func TestManager_Init_DiscordMissingConfig(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	viper.Set("notifications.discord.enabled", true)
	t.Setenv("DISCORD_BOT_TOKEN", "") // Missing token

	logCh := make(chan string, 1)
	logger := func(fmtStr string, args ...interface{}) {
		logCh <- fmtStr
	}

	NewManager(logger)

	select {
	case msg := <-logCh:
		assert.Contains(t, msg, "discord notifications disabled")
	case <-time.After(1 * time.Second):
		t.Error("Timed out waiting for warning log")
	}
}

func TestManager_Start(t *testing.T) {
	logCh := make(chan string, 1)
	logger := func(fmtStr string, args ...interface{}) {
		logCh <- fmtStr // simplified for test
	}

	m := &Manager{
		logger: logger,
	}

	// Create a real socketmode client structure
	api := slack.New("xoxb-test", slack.OptionAppLevelToken("xapp-test"))
	m.socketClient = socketmode.New(api)

	// Context that is already done to ensure RunContext returns immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	m.Start(ctx)

	// Wait for log
	select {
	case msg := <-logCh:
		assert.Contains(t, msg, "Starting Slack Socket Mode")
	case <-time.After(1 * time.Second):
		t.Error("Timed out waiting for start log")
	}
}
