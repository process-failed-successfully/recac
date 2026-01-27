package notify

import (
	"context"
	"errors"
	"testing"

	"github.com/slack-go/slack"
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

func TestManager_Init(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	// Enable both
	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.discord.enabled", true)

	// Set Env Vars
	t.Setenv("SLACK_BOT_USER_TOKEN", "xoxb-test-token")
	t.Setenv("SLACK_APP_TOKEN", "xapp-test-token")
	t.Setenv("DISCORD_BOT_TOKEN", "discord-test-token")
	t.Setenv("DISCORD_CHANNEL_ID", "123456")

	m := NewManager(func(fmt string, args ...interface{}) {})

	// Assertions
	assert.NotNil(t, m)
	assert.NotNil(t, m.client, "Slack client should be initialized")
	assert.NotNil(t, m.socketClient, "Slack socket client should be initialized due to xapp- token")
	assert.NotNil(t, m.discordNotifier, "Discord notifier should be initialized")

	// Test Start with valid client (though we can't wait for it as it blocks or connects)
	// Just call it to ensure no panic if client is set
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately so we don't block or leak
	m.Start(ctx)
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
		expected string // Color
	}{
		{EventStart, "#3498db"},
		{EventSuccess, "#2eb886"},
		{EventFailure, "#a30200"},
		{EventUserInteraction, "#f1c40f"},
		{EventProjectComplete, "#2eb886"},
		{"unknown_event", "#808080"},
	}

	for _, tt := range tests {
		title, color := getStyle(tt.event)
		assert.NotEmpty(t, title)
		assert.Equal(t, tt.expected, color)
	}
}

func TestManager_Init_DiscordWarning(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	viper.Set("notifications.discord.enabled", true)
	// Missing tokens

	logMsg := ""
	logger := func(fmt string, args ...interface{}) {
		logMsg = fmt
	}

	NewManager(logger)

	assert.Contains(t, logMsg, "Warning: DISCORD_BOT_TOKEN or DISCORD_CHANNEL_ID not set")
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
