package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DiscordNotifier sends notifications to Discord via Webhook or Bot API.
type DiscordNotifier struct {
	WebhookURL string
	BotToken   string
	ChannelID  string
	BaseURL    string // For testing mostly, defaults to https://discord.com
	Client     *http.Client
}

// NewDiscordNotifier creates a new DiscordNotifier using Webhook.
func NewDiscordNotifier(webhookURL string) *DiscordNotifier {
	return &DiscordNotifier{
		WebhookURL: webhookURL,
		Client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// NewDiscordBotNotifier creates a new DiscordNotifier using Bot Token.
func NewDiscordBotNotifier(token, channelID string) *DiscordNotifier {
	return &DiscordNotifier{
		BotToken:  token,
		ChannelID: channelID,
		BaseURL:   "https://discord.com",
		Client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Notify sends a message to the configured Discord webhook or channel.
// This is the legacy interface method.
func (n *DiscordNotifier) Notify(ctx context.Context, message string) error {
	_, err := n.Send(ctx, message, "")
	return err
}

// Send sends a message to Discord and returns the Message ID (if using Bot API).
// threadID is used for replying (Message Reference) in Bot API.
func (n *DiscordNotifier) Send(ctx context.Context, message, threadID string) (string, error) {
	// 1. Bot API (Preferred if configured)
	if n.BotToken != "" && n.ChannelID != "" {
		return n.sendBotMessage(ctx, message, threadID)
	}

	// 2. Webhook Fallback
	if n.WebhookURL != "" {
		return "", n.sendWebhookMessage(ctx, message)
	}

	return "", fmt.Errorf("discord not configured (missing token/channel or webhook)")
}

func (n *DiscordNotifier) sendWebhookMessage(ctx context.Context, message string) error {
	payload := map[string]string{
		"content": message,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal discord payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", n.WebhookURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create discord request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := n.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send discord notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord notification failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (n *DiscordNotifier) sendBotMessage(ctx context.Context, message, replyToID string) (string, error) {
	baseURL := n.BaseURL
	if baseURL == "" {
		baseURL = "https://discord.com"
	}
	url := fmt.Sprintf("%s/api/v10/channels/%s/messages", baseURL, n.ChannelID)

	payload := map[string]interface{}{
		"content": message,
	}

	if replyToID != "" {
		payload["message_reference"] = map[string]string{
			"message_id": replyToID,
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal discord payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create discord request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bot "+n.BotToken)

	resp, err := n.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send discord message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Try to read body for error details
		respBody, _ := createResponseError(resp)
		return "", respBody
	}

	// Parse Response to get ID
	var respData struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return "", fmt.Errorf("failed to decode discord response: %w", err)
	}

	return respData.ID, nil
}

// AddReaction adds an emoji reaction to a message.
// Note: reaction must be URL encoded if it's unicode, or name:id for custom emojis.
func (n *DiscordNotifier) AddReaction(ctx context.Context, messageID, reaction string) error {
	if n.BotToken == "" || n.ChannelID == "" {
		return fmt.Errorf("bot token and channel id required for reactions")
	}

	baseURL := n.BaseURL
	if baseURL == "" {
		baseURL = "https://discord.com"
	}

	// Map common Slack emojis to Discord equivalents
	reaction = mapEmoji(reaction)

	url := fmt.Sprintf("%s/api/v10/channels/%s/messages/%s/reactions/%s/@me", baseURL, n.ChannelID, messageID, reaction)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create reaction request: %w", err)
	}

	req.Header.Set("Authorization", "Bot "+n.BotToken)

	resp, err := n.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to add reaction: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := createResponseError(resp)
		return respBody
	}

	return nil
}

func mapEmoji(slackEmoji string) string {
	switch slackEmoji {
	case "white_check_mark", ":white_check_mark:":
		return "%E2%9C%85" // ✅
	case "x", ":x:":
		return "%E2%9D%8C" // ❌
	case "warning", ":warning:":
		return "%E2%9A%A0%EF%B8%8F" // ⚠️
	default:
		// Return as is (hoping it's compatible or user provided valid one)
		// Usually unicode needs to be URL encoded.
		return slackEmoji
	}
}

func createResponseError(resp *http.Response) (error, bool) {
	// dummy bool return to match signature style if needed, but here just error
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	return fmt.Errorf("discord api error: %d - %s", resp.StatusCode, buf.String()), true
}
