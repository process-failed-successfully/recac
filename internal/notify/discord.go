package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DiscordNotifier sends notifications to a Discord webhook.
type DiscordNotifier struct {
	WebhookURL string
	Client     *http.Client
}

// NewDiscordNotifier creates a new DiscordNotifier.
func NewDiscordNotifier(webhookURL string) *DiscordNotifier {
	return &DiscordNotifier{
		WebhookURL: webhookURL,
		Client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Notify sends a message to the configured Discord webhook.
func (n *DiscordNotifier) Notify(ctx context.Context, message string) error {
	if n.WebhookURL == "" {
		return fmt.Errorf("discord webhook URL is not configured")
	}

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
