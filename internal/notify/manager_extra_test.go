package notify

import (
	"context"
	"errors"
	"testing"

	"github.com/slack-go/slack"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestManager_Notify_PartialSuccess(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.slack.events.on_start", true)
	viper.Set("notifications.discord.enabled", true)

	mockSlack := &mockSlackPoster{
		postMessageContextFunc: func(ctx context.Context, channelID string, options ...slack.MsgOption) (string, string, error) {
			return "channel", "slack_ts_1", nil
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
		logger:          func(fmt string, args ...interface{}) {},
	}

	ctx := context.Background()
	state, err := m.Notify(ctx, EventStart, "message", "")

	assert.NoError(t, err)
	// Slack succeeded, Discord failed.
	// State should contain Slack TS but no Discord ID.
	// Optimization in manager.go returns plain string if only Slack TS is present
	assert.Equal(t, "slack_ts_1", state)
}

func TestManager_Notify_PartialSuccess_DiscordOnly(t *testing.T) {
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
			return "discord_id_1", nil
		},
	}

	m := &Manager{
		client:          mockSlack,
		discordNotifier: mockDiscord,
		channelID:       "#test",
		logger:          func(fmt string, args ...interface{}) {},
	}

	ctx := context.Background()
	state, err := m.Notify(ctx, EventStart, "message", "")

	assert.NoError(t, err)
	// Slack failed, Discord succeeded.
	assert.Contains(t, state, `"discord_id":"discord_id_1"`)
	assert.NotContains(t, state, "slack_ts")
}

func TestManager_AddReaction_Partial(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	mockSlack := &mockSlackPoster{
		addReactionContextFunc: func(ctx context.Context, name string, item slack.ItemRef) error {
			return errors.New("slack fail")
		},
	}

	mockDiscord := &mockDiscordPoster{
		addReactionFunc: func(ctx context.Context, messageID, reaction string) error {
			return nil
		},
	}

	m := &Manager{
		client:          mockSlack,
		discordNotifier: mockDiscord,
		channelID:       "#test",
		logger:          func(fmt string, args ...interface{}) {},
	}

	threadState := `{"slack_ts":"ts_1","discord_id":"did_1"}`
	err := m.AddReaction(context.Background(), threadState, "thumbsup")
	assert.NoError(t, err) // Should not return error even if one fails, as it logs errors internally
}

func TestManager_Start_NoSocket(t *testing.T) {
	// Just verify it doesn't panic when socketClient is nil
	m := &Manager{
		logger: func(fmt string, args ...interface{}) {},
	}
	m.Start(context.Background())
}
