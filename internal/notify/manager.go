package notify

import (
	"context"
	"encoding/json"
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

// Manager handles notifications across different providers (Slack and Discord).
type Manager struct {
	// Slack
	client       *slack.Client
	socketClient *socketmode.Client
	channelID    string

	// Discord
	discordNotifier *DiscordNotifier

	logger func(string, ...interface{})
}

// ThreadState represents the state of threads across providers
type ThreadState struct {
	SlackTS   string `json:"slack_ts,omitempty"`
	DiscordID string `json:"discord_id,omitempty"`
}

// NewManager creates a new Notification Manager.
func NewManager(logger func(string, ...interface{})) *Manager {
	m := &Manager{
		logger: logger,
	}

	// Initialize Slack
	m.initSlack()

	// Initialize Discord
	m.initDiscord()

	return m
}

func (m *Manager) initSlack() {
	if !viper.GetBool("notifications.slack.enabled") {
		return
	}

	botToken := os.Getenv("SLACK_BOT_USER_TOKEN")
	appToken := os.Getenv("SLACK_APP_TOKEN")

	if botToken == "" {
		if m.logger != nil {
			m.logger("Warning: SLACK_BOT_USER_TOKEN not set, slack notifications disabled")
		}
		return
	}

	// Initialize API Client
	api := slack.New(
		botToken,
		slack.OptionAppLevelToken(appToken),
	)

	m.client = api
	m.channelID = viper.GetString("notifications.slack.channel")

	if appToken != "" && strings.HasPrefix(appToken, "xapp-") {
		m.socketClient = socketmode.New(api)
	}
}

func (m *Manager) initDiscord() {
	if !viper.GetBool("notifications.discord.enabled") {
		return
	}

	botToken := os.Getenv("DISCORD_BOT_TOKEN")
	channelID := os.Getenv("DISCORD_CHANNEL_ID")
	if channelID == "" {
		channelID = viper.GetString("notifications.discord.channel")
	}

	if botToken != "" && channelID != "" {
		m.discordNotifier = NewDiscordBotNotifier(botToken, channelID)
	} else {
		// Fallback to webhook if bot token missing but webhook url exists?
		// User didn't ask for webhook support in Manager, but DiscordNotifier supports it.
		// For now we prioritize Bot for "parity".
		if m.logger != nil {
			m.logger("Warning: DISCORD_BOT_TOKEN or DISCORD_CHANNEL_ID not set, discord notifications disabled")
		}
	}
}

// Start initiates background clients (e.g. Socket Mode) if configured.
func (m *Manager) Start(ctx context.Context) {
	if m.socketClient != nil {
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
}

// Notify sends a notification if the event is enabled in configuration.
// It returns a JSON string containing thread IDs for active providers.
func (m *Manager) Notify(ctx context.Context, eventType string, message string, threadStateStr string) (string, error) {
	if m.logger != nil {
		m.logger("Checking notification for event: %s", eventType)
	}

	if !m.isEnabled(eventType) {
		return "", nil
	}

	if m.logger != nil {
		m.logger("Sending notification for event: %s", eventType)
	}

	// Parse Thread State
	ts := parseThreadState(threadStateStr)

	// Send to Slack
	if m.client != nil && m.isProviderEnabled("slack") {
		newTS, err := m.notifySlack(ctx, message, ts.SlackTS)
		if err != nil {
			if m.logger != nil {
				m.logger("Failed to send Slack notification: %v", err)
			}
		} else {
			ts.SlackTS = newTS
		}
	}

	// Send to Discord
	if m.discordNotifier != nil && m.isProviderEnabled("discord") {
		newID, err := m.discordNotifier.Send(ctx, message, ts.DiscordID)
		if err != nil {
			if m.logger != nil {
				m.logger("Failed to send Discord notification: %v", err)
			}
		} else {
			ts.DiscordID = newID
		}
	}

	// Return updated state as JSON string
	return dumpThreadState(ts), nil
}

func (m *Manager) notifySlack(ctx context.Context, message, threadTS string) (string, error) {
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

	_, newTS, err := m.client.PostMessageContext(ctx, channelID, opts...)
	if err != nil {
		return "", err
	}
	return newTS, nil
}

func (m *Manager) isEnabled(eventType string) bool {
	// Check global enabled (if any provider is enabled)
	slackEnabled := m.isProviderEnabled("slack")
	discordEnabled := m.isProviderEnabled("discord")

	if !slackEnabled && !discordEnabled {
		return false
	}

	// Check event specific (using slack config structure as default/shared for now, or check both?)
	// To keep it simple, we assume event configuration is shared or "slack" key implies generic "notifications".
	// But ideally we check `notifications.events.on_start` or `notifications.slack.events.on_start`.
	// Current config uses `notifications.slack.events.*`.
	// We will use that as the master switch for events for now.
	eventEnabled := viper.GetBool("notifications.slack.events." + eventType)
	return eventEnabled
}

func (m *Manager) isProviderEnabled(provider string) bool {
	return viper.GetBool("notifications." + provider + ".enabled")
}

// AddReaction adds an emoji reaction to a message.
func (m *Manager) AddReaction(ctx context.Context, threadStateStr, reaction string) error {
	ts := parseThreadState(threadStateStr)

	// Slack
	if m.client != nil && ts.SlackTS != "" {
		channelID := m.channelID
		if channelID == "" {
			channelID = "#general"
		}
		err := m.client.AddReactionContext(ctx, reaction, slack.ItemRef{
			Channel:   channelID,
			Timestamp: ts.SlackTS,
		})
		if err != nil && m.logger != nil {
			m.logger("Failed to add Slack reaction %s: %v", reaction, err)
		}
	}

	// Discord
	if m.discordNotifier != nil && ts.DiscordID != "" {
		err := m.discordNotifier.AddReaction(ctx, ts.DiscordID, reaction)
		if err != nil && m.logger != nil {
			m.logger("Failed to add Discord reaction %s: %v", reaction, err)
		}
	}

	return nil
}

// Helpers for Thread State

func parseThreadState(s string) ThreadState {
	var ts ThreadState
	if s == "" {
		return ts
	}

	// Try parsing as JSON
	if err := json.Unmarshal([]byte(s), &ts); err == nil {
		return ts
	}

	// Fallback: Treat as legacy Slack TS (string)
	return ThreadState{SlackTS: s}
}

func dumpThreadState(ts ThreadState) string {
	// If only Slack is present, return just the string for backward compatibility?
	// No, we should probably stick to JSON if possible, but Session expects a string.
	// If we change format mid-flight, existing sessions might be confused?
	// But Session just passes it back to Notify.
	// So we can change format.
	// HOWEVER: If we return JSON, next time parseThreadState will work.

	// Optimization: If only Slack is used, return plain string?
	// This helps readability in logs.
	if ts.DiscordID == "" && ts.SlackTS != "" {
		return ts.SlackTS
	}

	// If both or just Discord, use JSON
	data, _ := json.Marshal(ts)
	return string(data)
}
