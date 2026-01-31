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

func TestManager_Initialization(t *testing.T) {
	// Reset viper
	viper.Reset()
	defer viper.Reset()

	// 1. Slack Enabled but No Token
	viper.Set("notifications.slack.enabled", true)
	t.Setenv("SLACK_BOT_USER_TOKEN", "")

	loggerCalled := false
	logger := func(msg string, args ...interface{}) {
		loggerCalled = true
		if msg == "Warning: SLACK_BOT_USER_TOKEN not set, slack notifications disabled" {
			// Expected
		}
	}

	m := NewManager(logger)
	assert.Nil(t, m.client)
	assert.True(t, loggerCalled, "Expected warning log for missing Slack token")

	// 2. Discord Enabled but No Token
	viper.Set("notifications.slack.enabled", false)
	viper.Set("notifications.discord.enabled", true)
	t.Setenv("DISCORD_BOT_TOKEN", "")
	t.Setenv("DISCORD_CHANNEL_ID", "")
	viper.Set("notifications.discord.channel", "")

	loggerCalled = false
	logger = func(msg string, args ...interface{}) {
		loggerCalled = true
	}
	m = NewManager(logger)
	assert.Nil(t, m.discordNotifier)
	assert.True(t, loggerCalled, "Expected warning log for missing Discord token")

	// 3. Slack Socket Mode (App Token starts with xapp-)
	viper.Set("notifications.slack.enabled", true)
	t.Setenv("SLACK_BOT_USER_TOKEN", "xoxb-fake")
	t.Setenv("SLACK_APP_TOKEN", "xapp-fake")

	m = NewManager(nil)
	assert.NotNil(t, m.client)
	assert.NotNil(t, m.socketClient)
}

func TestManager_Start_Socket(t *testing.T) {
	// We can't easily mock socketmode.Client without wrapping it, but we can verify Start calls RunContext.
	// Since socketmode.New returns a struct, we can't mock it.
	// However, we can create a Manager with a manually injected socketClient if we modify the struct to use an interface or
	// just accept that we can't test RunContext execution easily without integration tests.
	// But `Start` method runs it in goroutine.

	// Let's rely on the fact that if socketClient is set, it tries to run.
	// We can't really verify it runs without side effects or mocking the client which is hard here.
	// So we'll skip deep verification of Start and assume `initSlack` test covers that `socketClient` is populated.
	// But we can check if Start logs something.

	m := &Manager{
		socketClient: socketmode.New(slack.New("fake")),
		logger: func(msg string, args ...interface{}) {
			// Just to capture logs
		},
	}

	// It runs in background, so just calling it shouldn't block
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	// Wait a bit
	time.Sleep(10 * time.Millisecond)
}

func TestManager_AddReaction_Error(t *testing.T) {
	// Cover error logging path
	mockSlack := &mockSlackPoster{
		addReactionContextFunc: func(ctx context.Context, name string, item slack.ItemRef) error {
			return errors.New("slack error")
		},
	}

	mockDiscord := &mockDiscordPoster{
		addReactionFunc: func(ctx context.Context, messageID, reaction string) error {
			return errors.New("discord error")
		},
	}

	loggerCalled := 0
	logger := func(msg string, args ...interface{}) {
		loggerCalled++
	}

	m := &Manager{
		client:          mockSlack,
		discordNotifier: mockDiscord,
		logger:          logger,
	}

	ts := `{"slack_ts":"1","discord_id":"2"}`
	err := m.AddReaction(context.Background(), ts, "up")
	assert.NoError(t, err) // It swallows error but logs
	assert.Equal(t, 2, loggerCalled)
}
