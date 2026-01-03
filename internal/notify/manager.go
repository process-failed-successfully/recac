package notify

import (
	"context"
	"os"
	"strings"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
	"github.com/spf13/viper"
)

// Event types
const (
	EventStart           = "on_start"
	EventSuccess         = "on_success"
	EventFailure         = "on_failure"
	EventUserInteraction = "on_user_interaction"
	EventProjectComplete = "on_project_complete"
)

// Manager handles notifications across different providers (currently only Slack).
type Manager struct {
	client       *slack.Client
	socketClient *socketmode.Client
	channelID    string
	logger       func(string, ...interface{})
}

// NewManager creates a new Notification Manager.
func NewManager(logger func(string, ...interface{})) *Manager {
	if !viper.GetBool("notifications.slack.enabled") {
		if os.Getenv("SLACK_BOT_USER_TOKEN") != "" && logger != nil {
			logger("Warning: SLACK_BOT_USER_TOKEN found but 'notifications.slack.enabled' is false in config. No notifications will be sent.")
		}
		return &Manager{logger: logger}
	}

	botToken := os.Getenv("SLACK_BOT_USER_TOKEN")
	appToken := os.Getenv("SLACK_APP_TOKEN")

	if botToken == "" {
		if logger != nil {
			logger("Warning: SLACK_BOT_USER_TOKEN not set, notifications disabled")
		}
		return &Manager{logger: logger}
	}

	// Initialize API Client
	api := slack.New(
		botToken,
		slack.OptionAppLevelToken(appToken),
		// slack.OptionDebug(viper.GetBool("verbose")),
		// slack.OptionLog(log.New(os.Stdout, "slack-bot: ", log.Lshortfile|log.LstdFlags)),
	)

	m := &Manager{
		client:    api,
		channelID: viper.GetString("notifications.slack.channel"),
		logger:    logger,
	}

	if appToken != "" && strings.HasPrefix(appToken, "xapp-") {
		m.socketClient = socketmode.New(
			api,
			// socketmode.OptionDebug(viper.GetBool("verbose")),
			// socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
		)
	}

	return m
}

// Start initiates the Socket Mode client in a background goroutine if configured.
func (m *Manager) Start(ctx context.Context) {
	if m.socketClient == nil {
		return
	}

	go func() {
		if m.logger != nil {
			m.logger("Starting Slack Socket Mode...")
		}

		err := m.socketClient.RunContext(ctx)
		if err != nil && err != context.Canceled {
			if m.logger != nil {
				m.logger("Slack Socket Mode error: %v", err)
			}
		}
	}()
}

// Notify sends a notification if the event is enabled in configuration.
// It returns the timestamp of the sent message (for threading) and an error.
func (m *Manager) Notify(ctx context.Context, eventType string, message string, threadTS string) (string, error) {
	if m.logger != nil {
		m.logger("Checking notification for event: %s", eventType)
	}

	if !m.isEnabled(eventType) {
		if m.logger != nil {
			m.logger("Notification DISABLED for event: %s", eventType)
		}
		return "", nil
	}

	if m.logger != nil {
		m.logger("Sending notification for event: %s", eventType)
	}

	if m.client != nil {
		// Use channel from config, default to user providing a valid channel name/ID
		channelID := m.channelID
		if channelID == "" {
			channelID = "#general"
		}

		opts := []slack.MsgOption{
			slack.MsgOptionText(message, false),
		}

		if threadTS != "" {
			opts = append(opts, slack.MsgOptionTS(threadTS))
		}

		_, ts, err := m.client.PostMessageContext(ctx, channelID, opts...)
		if err != nil {
			if m.logger != nil {
				m.logger("Failed to send Slack notification: %v", err)
			}
			return "", err
		}
		return ts, nil
	}
	return "", nil
}

func (m *Manager) isEnabled(eventType string) bool {
	enabled := viper.GetBool("notifications.slack.enabled")
	eventEnabled := viper.GetBool("notifications.slack.events." + eventType)

	// DEBUG: Remove after fixing
	// fmt.Printf("DEBUG: slack.enabled=%v, events.%s=%v\n", enabled, eventType, eventEnabled)

	if !enabled {
		return false
	}
	return eventEnabled
}

// AddReaction adds an emoji reaction to a message.
func (m *Manager) AddReaction(ctx context.Context, timestamp, reaction string) error {
	if m.client == nil {
		return nil
	}

	channelID := m.channelID
	if channelID == "" {
		channelID = "#general"
	}

	// This is a "nice to have" feature, so we log errors but don't fail hard if it fails
	// (e.g. if bot doesn't have permissions)
	err := m.client.AddReactionContext(ctx, reaction, slack.ItemRef{
		Channel:   channelID,
		Timestamp: timestamp,
	})

	if err != nil {
		if m.logger != nil {
			m.logger("Failed to add reaction %s: %v", reaction, err)
		}
		return err
	}

	return nil
}
