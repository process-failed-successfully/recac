package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscordNotifier_Notify(t *testing.T) {
	receivedContent := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST request, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		var payload map[string]string
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &payload)
		receivedContent = payload["content"]

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewDiscordNotifier(server.URL)
	ctx := context.Background()
	message := "Hello Discord!"

	err := notifier.Notify(ctx, message)
	if err != nil {
		t.Fatalf("Notify failed: %v", err)
	}

	if receivedContent != message {
		t.Errorf("expected content %q, got %q", message, receivedContent)
	}
}

func TestDiscordNotifier_Notify_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	notifier := NewDiscordNotifier(server.URL)
	ctx := context.Background()

	err := notifier.Notify(ctx, "test")
	if err == nil {
		t.Error("expected error for non-OK status code, got nil")
	}
}

func TestDiscordNotifier_Notify_MissingURL(t *testing.T) {
	notifier := NewDiscordNotifier("")
	ctx := context.Background()

	err := notifier.Notify(ctx, "test")
	if err == nil {
		t.Error("expected error for missing webhook URL, got nil")
	}
}

func TestDiscordNotifier_Send_Bot(t *testing.T) {
	channelID := "12345"
	expectedID := "msg_123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bot my_token" {
			t.Errorf("missing or invalid authorization header")
		}

		// Since we use BaseURL, the path will be /channels/...
		if !strings.Contains(r.URL.Path, "/channels/"+channelID) {
			t.Errorf("expected channel ID in path, got %s", r.URL.Path)
		}

		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		if payload["content"] != "Hello Bot" {
			t.Errorf("unexpected content")
		}

		// If thread ID provided
		if ref, ok := payload["message_reference"].(map[string]interface{}); ok {
			if ref["message_id"] != "thread_123" {
				t.Errorf("expected thread ID thread_123, got %v", ref["message_id"])
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": expectedID})
	}))
	defer server.Close()

	notifier := NewDiscordBotNotifier("my_token", channelID)
	// Set BaseURL to test server
	notifier.BaseURL = server.URL

	ctx := context.Background()
	id, err := notifier.Send(ctx, "Hello Bot", "thread_123")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if id != expectedID {
		t.Errorf("expected message ID %s, got %s", expectedID, id)
	}
}

func TestDiscordNotifier_Send_Bot_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("some error from discord"))
	}))
	defer server.Close()

	notifier := NewDiscordBotNotifier("token", "chan")
	notifier.BaseURL = server.URL

	_, err := notifier.Send(context.Background(), "msg", "")
	if err == nil {
		t.Error("expected error")
	}
	if !strings.Contains(err.Error(), "some error from discord") {
		t.Errorf("expected error message to contain 'some error from discord', got %v", err)
	}
}

func TestDiscordNotifier_AddReaction(t *testing.T) {
	channelID := "12345"
	messageID := "msg_123"

	tests := []struct {
		inputEmoji         string
		expectedEmojiInURL string
	}{
		{"white_check_mark", "✅"}, // ✅
		{":x:", "❌"},              // ❌
		{"custom:123", "custom:123"},
	}

	for _, tt := range tests {
		t.Run(tt.inputEmoji, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "PUT" {
					t.Errorf("expected PUT request, got %s", r.Method)
				}

				// Check reaction in URL
				if !strings.Contains(r.URL.Path, tt.expectedEmojiInURL) {
					t.Errorf("expected reaction %s in URL, got %s", tt.expectedEmojiInURL, r.URL.Path)
				}

				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			notifier := NewDiscordBotNotifier("my_token", channelID)
			notifier.BaseURL = server.URL

			ctx := context.Background()
			err := notifier.AddReaction(ctx, messageID, tt.inputEmoji)
			if err != nil {
				t.Fatalf("AddReaction failed: %v", err)
			}
		})
	}
}

func TestDiscordNotifier_AddReaction_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Forbidden"))
	}))
	defer server.Close()

	notifier := NewDiscordBotNotifier("token", "chan")
	notifier.BaseURL = server.URL

	err := notifier.AddReaction(context.Background(), "msg", "smile")
	if err == nil {
		t.Error("expected error")
	}
}
