package notify

import (
	"context"
	"testing"
	"net/http"

	"github.com/stretchr/testify/assert"
)

func TestDiscordNotifier_MapEmoji(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"white_check_mark", "%E2%9C%85"},
		{":white_check_mark:", "%E2%9C%85"},
		{"x", "%E2%9D%8C"},
		{":x:", "%E2%9D%8C"},
		{"warning", "%E2%9A%A0%EF%B8%8F"},
		{":warning:", "%E2%9A%A0%EF%B8%8F"},
		{"other", "other"},
		{"unicode-✅", "unicode-✅"}, // Not encoded by mapEmoji if not matched
	}

	for _, tc := range tests {
		assert.Equal(t, tc.expected, mapEmoji(tc.input))
	}
}

func TestDiscordNotifier_Send_Webhook_RequestError(t *testing.T) {
	// Invalid URL (control character)
	notifier := NewDiscordNotifier("http://example.com/\x00")
	_, err := notifier.Send(context.Background(), "msg", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create discord request")
}

func TestDiscordNotifier_Send_Bot_RequestError(t *testing.T) {
	notifier := NewDiscordBotNotifier("token", "channel\x00") // Invalid channel ID causing invalid URL
	_, err := notifier.Send(context.Background(), "msg", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create discord request")
}

func TestDiscordNotifier_AddReaction_RequestError(t *testing.T) {
	notifier := NewDiscordBotNotifier("token", "channel\x00")
	err := notifier.AddReaction(context.Background(), "msg", "smile")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create reaction request")
}

// Test sendWebhookMessage with Client.Do error (mock transport failure)
func TestDiscordNotifier_Send_Webhook_ClientError(t *testing.T) {
	notifier := NewDiscordNotifier("http://example.com")
	// Inject failing client
	notifier.Client = &http.Client{
		Transport: &failingTransport{},
	}

	_, err := notifier.Send(context.Background(), "msg", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send discord notification")
}

type failingTransport struct{}

func (t *failingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, assert.AnError
}
