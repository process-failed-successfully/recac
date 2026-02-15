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

func TestManager_Start_Coverage(t *testing.T) {
	// Create a dummy socket client
	api := slack.New("dummy-token")
	sc := socketmode.New(api)

	m := &Manager{
		socketClient: sc,
		logger: func(msg string, args ...interface{}) {
			// dummy logger
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start spawns a goroutine
	m.Start(ctx)

	// Sleep briefly to allow goroutine to start and hit RunContext
	// Since we can't sync, this is best effort for coverage
	time.Sleep(10 * time.Millisecond)
}

func TestManager_GetStyle_Coverage(t *testing.T) {
	tests := []struct {
		event string
		color string
	}{
		{EventStart, "#3498db"},
		{EventSuccess, "#2eb886"},
		{EventFailure, "#a30200"},
		{EventUserInteraction, "#f1c40f"},
		{EventProjectComplete, "#2eb886"},
		{"unknown", "#808080"},
	}

	for _, tt := range tests {
		_, color := getStyle(tt.event)
		assert.Equal(t, tt.color, color, "Color mismatch for event %s", tt.event)
	}
}

func TestAddReaction_ErrorPaths(t *testing.T) {
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

	m := &Manager{
		client:          mockSlack,
		discordNotifier: mockDiscord,
		channelID:       "#test",
		logger:          func(fmt string, args ...interface{}) {},
	}

	// Should not panic or return error (it logs errors)
	err := m.AddReaction(context.Background(), `{"slack_ts":"1","discord_id":"2"}`, "thumbsup")
	assert.NoError(t, err)
}

func TestManager_Notify_PartialFailure(t *testing.T) {
	// Reset viper
	viper.Reset()
	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.discord.enabled", true)
	viper.Set("notifications.slack.events.on_start", true)

	// Slack fails, Discord succeeds
	mockSlack := &mockSlackPoster{
		postMessageContextFunc: func(ctx context.Context, channelID string, options ...slack.MsgOption) (string, string, error) {
			return "", "", errors.New("slack error")
		},
	}
	mockDiscord := &mockDiscordPoster{
		sendFunc: func(ctx context.Context, message, threadID string) (string, error) {
			return "discord_id_1", nil
		},
	}

	m := &Manager{
		client:          mockSlack,
		discordNotifier: mockDiscord,
		channelID:       "#test",
		logger:          func(fmt string, args ...interface{}) {},
	}

	// Should not panic or return error
	threadState, err := m.Notify(context.Background(), EventStart, "msg", "")
	assert.NoError(t, err)
	assert.Contains(t, threadState, `"discord_id":"discord_id_1"`)
}
