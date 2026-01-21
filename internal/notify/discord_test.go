package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiscordNotifier_Send_Webhook(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/webhook", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var payload map[string]string
		err := json.NewDecoder(r.Body).Decode(&payload)
		assert.NoError(t, err)
		assert.Equal(t, "Hello Webhook", payload["content"])

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewDiscordNotifier(server.URL + "/webhook")
	_, err := notifier.Send(context.Background(), "Hello Webhook", "")
	assert.NoError(t, err)
}

func TestDiscordNotifier_Send_Webhook_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	notifier := NewDiscordNotifier(server.URL)
	_, err := notifier.Send(context.Background(), "Hello", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "discord notification failed with status: 500")
}

func TestDiscordNotifier_Send_Bot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v10/channels/123/messages", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bot test-token", r.Header.Get("Authorization"))

		var payload map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&payload)
		assert.NoError(t, err)
		assert.Equal(t, "Hello Bot", payload["content"])

		if ref, ok := payload["message_reference"]; ok {
			refMap := ref.(map[string]interface{})
			assert.Equal(t, "reply-id", refMap["message_id"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"id": "msg-id-1"})
	}))
	defer server.Close()

	notifier := NewDiscordBotNotifier("test-token", "123")
	notifier.BaseURL = server.URL // Override BaseURL for testing

	id, err := notifier.Send(context.Background(), "Hello Bot", "")
	assert.NoError(t, err)
	assert.Equal(t, "msg-id-1", id)

	// Test Reply
	id, err = notifier.Send(context.Background(), "Hello Bot", "reply-id")
	assert.NoError(t, err)
	assert.Equal(t, "msg-id-1", id)
}

func TestDiscordNotifier_Send_Bot_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message": "Unauthorized"}`))
	}))
	defer server.Close()

	notifier := NewDiscordBotNotifier("test-token", "123")
	notifier.BaseURL = server.URL

	_, err := notifier.Send(context.Background(), "Hello", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "discord api error: 401 - {\"message\": \"Unauthorized\"}")
}

func TestDiscordNotifier_AddReaction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Expected path: /api/v10/channels/123/messages/msg-id/reactions/%E2%9C%85/@me
		assert.Contains(t, r.URL.Path, "/api/v10/channels/123/messages/msg-id/reactions/")
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "Bot test-token", r.Header.Get("Authorization"))

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	notifier := NewDiscordBotNotifier("test-token", "123")
	notifier.BaseURL = server.URL

	err := notifier.AddReaction(context.Background(), "msg-id", "white_check_mark")
	assert.NoError(t, err)
}

func TestDiscordNotifier_AddReaction_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	notifier := NewDiscordBotNotifier("test-token", "123")
	notifier.BaseURL = server.URL

	err := notifier.AddReaction(context.Background(), "msg-id", "x")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "discord api error: 403")
}

func TestDiscordNotifier_Configuration(t *testing.T) {
	n := &DiscordNotifier{}
	_, err := n.Send(context.Background(), "fail", "")
	assert.Error(t, err)
	assert.Equal(t, "discord not configured (missing token/channel or webhook)", err.Error())

	err = n.AddReaction(context.Background(), "id", "x")
	assert.Error(t, err)
	assert.Equal(t, "bot token and channel id required for reactions", err.Error())
}

func TestDiscordNotifier_Notify(t *testing.T) {
	// Notify is just a wrapper around Send
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewDiscordNotifier(server.URL + "/webhook")
	err := notifier.Notify(context.Background(), "Hello")
	assert.NoError(t, err)
}

func TestMapEmoji(t *testing.T) {
	assert.Equal(t, "%E2%9C%85", mapEmoji("white_check_mark"))
	assert.Equal(t, "%E2%9C%85", mapEmoji(":white_check_mark:"))
	assert.Equal(t, "%E2%9D%8C", mapEmoji("x"))
	assert.Equal(t, "%E2%9D%8C", mapEmoji(":x:"))
	assert.Equal(t, "%E2%9A%A0%EF%B8%8F", mapEmoji("warning"))
	assert.Equal(t, "custom", mapEmoji("custom"))
}
