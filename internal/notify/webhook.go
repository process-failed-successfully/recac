package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WebhookNotifier handles sending notifications via generic HTTP POST requests.
type WebhookNotifier struct {
	url    string
	secret string
	client *http.Client
}

// WebhookPayload represents the JSON body sent to the webhook URL.
type WebhookPayload struct {
	Event   string `json:"event"`
	Message string `json:"message"`
}

// NewWebhookNotifier creates a new WebhookNotifier.
func NewWebhookNotifier(url, secret string) *WebhookNotifier {
	return &WebhookNotifier{
		url:    url,
		secret: secret,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Send sends an HTTP POST request to the webhook URL.
func (w *WebhookNotifier) Send(ctx context.Context, eventType, message string) error {
	payload := WebhookPayload{
		Event:   eventType,
		Message: message,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// If a secret is provided, calculate HMAC-SHA256 signature
	if w.secret != "" {
		mac := hmac.New(sha256.New, []byte(w.secret))
		mac.Write(bodyBytes)
		signature := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Webhook-Signature", "sha256="+signature)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned error status code: %d", resp.StatusCode)
	}

	return nil
}
