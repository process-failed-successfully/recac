package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// SlackNotifier sends notifications to Slack via a Webhook.
type SlackNotifier struct {
	WebhookURL string
}

// NewSlackNotifier creates a new SlackNotifier.
func NewSlackNotifier(webhookURL string) *SlackNotifier {
	return &SlackNotifier{
		WebhookURL: webhookURL,
	}
}

// Notify sends a message to the configured Slack webhook.
func (s *SlackNotifier) Notify(ctx context.Context, message string) error {
	if s.WebhookURL == "" {
		return fmt.Errorf("slack webhook URL is not configured")
	}

	payload := map[string]string{
		"text": message,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.WebhookURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send slack notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack notification failed with status: %s", resp.Status)
	}

	return nil
}
