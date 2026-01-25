package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockTransport struct {
	RoundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.RoundTripFunc != nil {
		return m.RoundTripFunc(req)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString("{}")),
	}, nil
}

func TestDiscordNotifier_Send_Webhook(t *testing.T) {
	n := NewDiscordNotifier("https://discord.com/api/webhooks/123/token")

	called := false
	n.Client.Transport = &mockTransport{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			called = true
			assert.Equal(t, "POST", req.Method)
			assert.Equal(t, "https://discord.com/api/webhooks/123/token", req.URL.String())

			var body map[string]string
			json.NewDecoder(req.Body).Decode(&body)
			assert.Equal(t, "test message", body["content"])

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("")),
			}, nil
		},
	}

	_, err := n.Send(context.Background(), "test message", "")
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestDiscordNotifier_Send_Bot(t *testing.T) {
	n := NewDiscordBotNotifier("bot-token", "channel-id")

	called := false
	n.Client.Transport = &mockTransport{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			called = true
			assert.Equal(t, "POST", req.Method)
			assert.Equal(t, "https://discord.com/api/v10/channels/channel-id/messages", req.URL.String())
			assert.Equal(t, "Bot bot-token", req.Header.Get("Authorization"))

			var body map[string]interface{}
			json.NewDecoder(req.Body).Decode(&body)
			assert.Equal(t, "test message", body["content"])

			respBody := `{"id": "msg-123"}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(respBody)),
			}, nil
		},
	}

	id, err := n.Send(context.Background(), "test message", "")
	assert.NoError(t, err)
	assert.Equal(t, "msg-123", id)
	assert.True(t, called)
}

func TestDiscordNotifier_AddReaction(t *testing.T) {
	n := NewDiscordBotNotifier("bot-token", "channel-id")

	tests := []struct {
		name     string
		emoji    string
		expected string
	}{
		{"check", ":white_check_mark:", "%E2%9C%85"},
		{"x", ":x:", "%E2%9D%8C"},
		{"warning", "warning", "%E2%9A%A0%EF%B8%8F"},
		{"other", "thumbsup", "thumbsup"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			n.Client.Transport = &mockTransport{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					called = true
					assert.Equal(t, "PUT", req.Method)
					expectedURL := fmt.Sprintf("https://discord.com/api/v10/channels/channel-id/messages/msg-1/reactions/%s/@me", tt.expected)
					assert.Equal(t, expectedURL, req.URL.String())
					return &http.Response{
						StatusCode: http.StatusNoContent,
						Body:       io.NopCloser(bytes.NewBufferString("")),
					}, nil
				},
			}

			err := n.AddReaction(context.Background(), "msg-1", tt.emoji)
			assert.NoError(t, err)
			assert.True(t, called)
		})
	}
}

func TestMapEmoji(t *testing.T) {
	assert.Equal(t, "%E2%9C%85", mapEmoji(":white_check_mark:"))
	assert.Equal(t, "%E2%9D%8C", mapEmoji(":x:"))
	assert.Equal(t, "%E2%9A%A0%EF%B8%8F", mapEmoji(":warning:"))
	assert.Equal(t, "foo", mapEmoji("foo"))
}
