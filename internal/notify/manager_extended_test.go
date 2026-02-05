package notify

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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

// MockDiscordPoster
type MockDiscordPoster struct {
	mock.Mock
}

func (m *MockDiscordPoster) Send(ctx context.Context, message, threadID string) (string, error) {
	args := m.Called(ctx, message, threadID)
	return args.String(0), args.Error(1)
}

func (m *MockDiscordPoster) AddReaction(ctx context.Context, messageID, reaction string) error {
	args := m.Called(ctx, messageID, reaction)
	return args.Error(0)
}

func TestManager_Notify_Mock(t *testing.T) {
	// Setup
	mockSlack := new(MockSlackPoster)
	mockDiscord := new(MockDiscordPoster)

	m := &Manager{
		client:          mockSlack,
		discordNotifier: mockDiscord,
		logger:          func(string, ...interface{}) {},
		channelID:       "#general",
	}

	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.discord.enabled", true)
	viper.Set("notifications.slack.events.on_test", true)
    t.Cleanup(func() {
        viper.Set("notifications.slack.enabled", false)
        viper.Set("notifications.discord.enabled", false)
        viper.Set("notifications.slack.events.on_test", false)
    })

	// Test 1: New notification (no thread state)
	ctx := context.Background()
	mockSlack.On("PostMessageContext", ctx, "#general", mock.Anything).Return("slack-ts-1", "slack-ts-1", nil).Once()
	mockDiscord.On("Send", ctx, "Hello", "").Return("discord-id-1", nil).Once()

	stateJSON, err := m.Notify(ctx, "on_test", "Hello", "")
	assert.NoError(t, err)

	var state ThreadState
	err = json.Unmarshal([]byte(stateJSON), &state)
	assert.NoError(t, err)
	assert.Equal(t, "slack-ts-1", state.SlackTS)
	assert.Equal(t, "discord-id-1", state.DiscordID)

	// Test 2: Reply (with thread state)
	mockSlack.On("PostMessageContext", ctx, "#general", mock.Anything).Return("slack-ts-2", "slack-ts-2", nil).Once()
	mockDiscord.On("Send", ctx, "Reply", "discord-id-1").Return("discord-id-2", nil).Once()

	stateJSON2, err := m.Notify(ctx, "on_test", "Reply", stateJSON)
	assert.NoError(t, err)

    var state2 ThreadState
    err = json.Unmarshal([]byte(stateJSON2), &state2)
    assert.Equal(t, "slack-ts-2", state2.SlackTS)
    assert.Equal(t, "discord-id-2", state2.DiscordID)

	mockSlack.AssertExpectations(t)
	mockDiscord.AssertExpectations(t)
}

func TestManager_AddReaction_Mock(t *testing.T) {
	mockSlack := new(MockSlackPoster)
	mockDiscord := new(MockDiscordPoster)

	m := &Manager{
		client:          mockSlack,
		discordNotifier: mockDiscord,
		logger:          func(string, ...interface{}) {},
		channelID:       "#general",
	}

	state := ThreadState{SlackTS: "ts1", DiscordID: "id1"}
	stateBytes, _ := json.Marshal(state)
	stateStr := string(stateBytes)

	ctx := context.Background()
	mockSlack.On("AddReactionContext", ctx, "check", slack.ItemRef{Channel: "#general", Timestamp: "ts1"}).Return(nil).Once()
	mockDiscord.On("AddReaction", ctx, "id1", "check").Return(nil).Once()

	err := m.AddReaction(ctx, stateStr, "check")
	assert.NoError(t, err)

	mockSlack.AssertExpectations(t)
	mockDiscord.AssertExpectations(t)
}

func TestManager_Init_Disabled(t *testing.T) {
    viper.Set("notifications.slack.enabled", false)
    viper.Set("notifications.discord.enabled", false)
    t.Cleanup(func() {
        viper.Reset()
    })

    m := NewManager(nil)
    assert.Nil(t, m.client)
    assert.Nil(t, m.discordNotifier)
}

func TestManager_Notify_DisabledEvent(t *testing.T) {
    m := &Manager{} // No clients
    viper.Set("notifications.slack.enabled", true) // Provider enabled
    viper.Set("notifications.slack.events.on_disabled", false) // Event disabled
    t.Cleanup(func() {
        viper.Reset()
    })

    resp, err := m.Notify(context.Background(), "on_disabled", "msg", "")
    assert.NoError(t, err)
    assert.Equal(t, "", resp)
}

func TestParseThreadState_Legacy(t *testing.T) {
	ts := parseThreadState("legacy-ts")
	assert.Equal(t, "legacy-ts", ts.SlackTS)
	assert.Empty(t, ts.DiscordID)
}

func TestDumpThreadState_SlackOnly(t *testing.T) {
	ts := ThreadState{SlackTS: "ts1"}
	str := dumpThreadState(ts)
	assert.Equal(t, "ts1", str)
}

func TestManager_Init_DiscordWarning(t *testing.T) {
	viper.Set("notifications.discord.enabled", true)
    t.Cleanup(func() { viper.Reset() })
	// Ensure env vars are unset
	t.Setenv("DISCORD_BOT_TOKEN", "")

	// We need to capture logger output to verify warning?
	// NewManager takes a logger func.
	var logs []string
	logger := func(msg string, args ...interface{}) {
		logs = append(logs, msg)
	}

	m := NewManager(logger)
	assert.Nil(t, m.discordNotifier)

	found := false
	for _, l := range logs {
		if l == "Warning: DISCORD_BOT_TOKEN or DISCORD_CHANNEL_ID not set, discord notifications disabled" {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected warning log not found")
}

func TestManager_Start_SocketMode(t *testing.T) {
	viper.Set("notifications.slack.enabled", true)
    t.Cleanup(func() { viper.Reset() })
	t.Setenv("SLACK_BOT_USER_TOKEN", "xoxb-test")
	t.Setenv("SLACK_APP_TOKEN", "xapp-test")

	m := NewManager(func(string, ...interface{}) {})
	assert.NotNil(t, m.socketClient)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately so Start returns/doesn't block forever

	m.Start(ctx)
	// Start launches a goroutine. We just verify it doesn't panic.
}

func TestManager_HandleEvents(t *testing.T) {
	// Create a socket client manually
	sc := socketmode.New(slack.New("xoxb-test"))
	// Events channel is buffered by default in New? No.
	// We can't write to sc.Events if we didn't make it?
	// socketmode.New creates the channel.

	m := &Manager{
		socketClient: sc,
		logger: func(format string, args ...interface{}) {
			// t.Logf(format, args...)
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	go m.HandleEvents(ctx)

	// Send an event
	sc.Events <- socketmode.Event{Type: socketmode.EventTypeConnected}

	// Close
	cancel()
}

func TestManager_GetStyle_All(t *testing.T) {
	tests := []struct {
		event string
		wantTitle string
	}{
		{EventStart, "🚀 Project Started"},
		{EventSuccess, "✅ Success"},
		{EventFailure, "❌ Failure"},
		{EventUserInteraction, "💬 Input Needed"},
		{EventProjectComplete, "🏁 Project Complete"},
		{"unknown", "📢 Notification"},
	}

	for _, tc := range tests {
		title, color := getStyle(tc.event)
		assert.Equal(t, tc.wantTitle, title)
		assert.NotEmpty(t, color)
	}
}
