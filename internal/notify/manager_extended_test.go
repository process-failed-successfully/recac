package notify

import (
	"context"
	"os"
	"testing"

	"github.com/slack-go/slack"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
)

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

func TestManager_Notify_SlackOnly(t *testing.T) {
	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.discord.enabled", false)
	viper.Set("notifications.slack.events.on_success", true)

	mockSlack := new(MockSlackPoster)

	m := &Manager{
		client: mockSlack,
		logger: func(msg string, args ...interface{}) {},
	}

	mockSlack.On("PostMessageContext", mock.Anything, mock.Anything, mock.Anything).Return("channel", "ts456", nil)

	stateStr, err := m.Notify(context.Background(), EventSuccess, "Success", "")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	// With only slack, dumpThreadState might return simple string depending on logic
	if stateStr != "ts456" {
		t.Errorf("Expected simple TS string 'ts456', got %s", stateStr)
	}
	mockSlack.AssertExpectations(t)
}

func TestManager_Init_WithSocket(t *testing.T) {
	// Save env
	oldBot := os.Getenv("SLACK_BOT_USER_TOKEN")
	oldApp := os.Getenv("SLACK_APP_TOKEN")
	defer func() {
		os.Setenv("SLACK_BOT_USER_TOKEN", oldBot)
		os.Setenv("SLACK_APP_TOKEN", oldApp)
	}()

	os.Setenv("SLACK_BOT_USER_TOKEN", "xoxb-test")
	os.Setenv("SLACK_APP_TOKEN", "xapp-test")

	viper.Set("notifications.slack.enabled", true)

	m := NewManager(func(msg string, args ...interface{}) {})

	if m.client == nil {
		t.Error("Expected Slack client to be initialized")
	}
	if m.socketClient == nil {
		t.Error("Expected Socket client to be initialized")
	}
}

func TestManager_Init_NoToken(t *testing.T) {
	os.Unsetenv("SLACK_BOT_USER_TOKEN")
	viper.Set("notifications.slack.enabled", true)

	m := NewManager(nil)

	if m.client != nil {
		t.Error("Expected Slack client to be nil without token")
	}
}

func TestManager_Init_Discord(t *testing.T) {
	oldBot := os.Getenv("DISCORD_BOT_TOKEN")
	oldChan := os.Getenv("DISCORD_CHANNEL_ID")
	defer func() {
		os.Setenv("DISCORD_BOT_TOKEN", oldBot)
		os.Setenv("DISCORD_CHANNEL_ID", oldChan)
	}()

	os.Setenv("DISCORD_BOT_TOKEN", "test-token")
	os.Setenv("DISCORD_CHANNEL_ID", "123")

	viper.Set("notifications.discord.enabled", true)

	m := NewManager(nil)

	if m.discordNotifier == nil {
		t.Error("Expected Discord notifier to be initialized")
	}
}
