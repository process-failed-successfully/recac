package notify

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockTransport struct {
	RoundTripFunc func(*http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.RoundTripFunc(req)
}

func TestDiscordNotifier_Send_Webhook_Success(t *testing.T) {
	n := NewDiscordNotifier("https://discord.com/api/webhooks/xxx")
	n.Client.Transport = &mockTransport{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "https://discord.com/api/webhooks/xxx", req.URL.String())
			assert.Equal(t, "POST", req.Method)

			body, _ := io.ReadAll(req.Body)
			assert.Contains(t, string(body), "hello")

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("{}")),
			}, nil
		},
	}

	id, err := n.Send(context.Background(), "hello", "")
	require.NoError(t, err)
	assert.Empty(t, id)
}

func TestDiscordNotifier_Send_Webhook_Error(t *testing.T) {
	n := NewDiscordNotifier("https://discord.com/api/webhooks/xxx")
	n.Client.Transport = &mockTransport{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(bytes.NewBufferString("Bad Request")),
			}, nil
		},
	}

	_, err := n.Send(context.Background(), "hello", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "discord notification failed with status: 400")
}

func TestDiscordNotifier_Send_Bot_Success(t *testing.T) {
	n := NewDiscordBotNotifier("mytoken", "123")
	n.Client.Transport = &mockTransport{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "https://discord.com/api/v10/channels/123/messages", req.URL.String())
			assert.Equal(t, "POST", req.Method)
			assert.Equal(t, "Bot mytoken", req.Header.Get("Authorization"))
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"id": "msg123"}`)),
			}, nil
		},
	}

	id, err := n.Send(context.Background(), "hello", "")
	require.NoError(t, err)
	assert.Equal(t, "msg123", id)
}

func TestDiscordNotifier_Send_Bot_WithReply(t *testing.T) {
	n := NewDiscordBotNotifier("mytoken", "123")
	n.Client.Transport = &mockTransport{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "https://discord.com/api/v10/channels/123/messages", req.URL.String())
			body, _ := io.ReadAll(req.Body)
			assert.Contains(t, string(body), `"message_reference":{"message_id":"reply123"}`)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"id": "msg456"}`)),
			}, nil
		},
	}

	id, err := n.Send(context.Background(), "hello", "reply123")
	require.NoError(t, err)
	assert.Equal(t, "msg456", id)
}

func TestDiscordNotifier_Send_Bot_Error(t *testing.T) {
	n := NewDiscordBotNotifier("mytoken", "123")
	n.Client.Transport = &mockTransport{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(bytes.NewBufferString(`{"message": "Unauthorized"}`)),
			}, nil
		},
	}

	_, err := n.Send(context.Background(), "hello", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "discord api error: 401 - {\"message\": \"Unauthorized\"}")
}

func TestDiscordNotifier_AddReaction_Success(t *testing.T) {
	n := NewDiscordBotNotifier("mytoken", "123")
	n.Client.Transport = &mockTransport{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "https://discord.com/api/v10/channels/123/messages/msg123/reactions/%E2%9C%85/@me", req.URL.String())
			assert.Equal(t, "PUT", req.Method)
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(bytes.NewBufferString("")),
			}, nil
		},
	}

	err := n.AddReaction(context.Background(), "msg123", "white_check_mark")
	require.NoError(t, err)
}

func TestDiscordNotifier_AddReaction_Error(t *testing.T) {
	n := NewDiscordBotNotifier("mytoken", "123")
	n.Client.Transport = &mockTransport{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Body:       io.NopCloser(bytes.NewBufferString("Forbidden")),
			}, nil
		},
	}

	err := n.AddReaction(context.Background(), "msg123", ":x:")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "discord api error: 403 - Forbidden")
}

func TestDiscordNotifier_NotConfigured(t *testing.T) {
	n := &DiscordNotifier{}
	_, err := n.Send(context.Background(), "hello", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "discord not configured")
}

func TestDiscordNotifier_Notify_Legacy(t *testing.T) {
	n := NewDiscordNotifier("https://discord.com/api/webhooks/xxx")
	n.Client.Transport = &mockTransport{
		RoundTripFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("{}")),
			}, nil
		},
	}

	err := n.Notify(context.Background(), "hello")
	require.NoError(t, err)
}
