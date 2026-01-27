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

		if !strings.Contains(r.URL.Path, channelID) {
			t.Errorf("expected channel ID in path")
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
	// Inject custom client that routes everything to test server
	notifier.Client = &http.Client{
		Transport: &testTransport{
			TargetURL: server.URL,
		},
	}

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
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	notifier := NewDiscordBotNotifier("my_token", "12345")
	notifier.Client = &http.Client{
		Transport: &testTransport{
			TargetURL: server.URL,
		},
	}

	ctx := context.Background()
	_, err := notifier.Send(ctx, "Hello Bot", "")
	if err == nil {
		t.Error("expected error for non-OK status code, got nil")
	}
	if !strings.Contains(err.Error(), "discord api error: 500") {
		t.Errorf("expected 500 error, got %v", err)
	}
}

func TestDiscordNotifier_Send_Bot_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	notifier := NewDiscordBotNotifier("my_token", "12345")
	notifier.Client = &http.Client{
		Transport: &testTransport{
			TargetURL: server.URL,
		},
	}

	ctx := context.Background()
	_, err := notifier.Send(ctx, "Hello Bot", "")
	if err == nil {
		t.Error("expected error for invalid json response, got nil")
	}
	if !strings.Contains(err.Error(), "failed to decode discord response") {
		t.Errorf("expected decode error, got %v", err)
	}
}

func TestDiscordNotifier_AddReaction(t *testing.T) {
	channelID := "12345"
	messageID := "msg_123"
	reaction := "white_check_mark"
	// mapEmoji converts "white_check_mark" to encoded unicode "✅" (%E2%9C%85)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT request, got %s", r.Method)
		}

		if r.Header.Get("Authorization") != "Bot my_token" {
			t.Errorf("missing or invalid authorization header")
		}

		// Check if URL contains the reaction
		// Ideally we check exact path but transport manipulation might affect it.
		// Just ensure it's calling the reaction endpoint
		if !strings.Contains(r.URL.Path, "/reactions/") {
			t.Errorf("expected reactions endpoint, got %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	notifier := NewDiscordBotNotifier("my_token", channelID)
	notifier.Client = &http.Client{
		Transport: &testTransport{
			TargetURL: server.URL,
		},
	}

	ctx := context.Background()
	err := notifier.AddReaction(ctx, messageID, reaction)
	if err != nil {
		t.Fatalf("AddReaction failed: %v", err)
	}
}

func TestDiscordNotifier_AddReaction_Error(t *testing.T) {
	// Case 1: Missing credentials
	notifier := NewDiscordBotNotifier("", "")
	ctx := context.Background()
	err := notifier.AddReaction(ctx, "msg_123", "check")
	if err == nil {
		t.Error("expected error for missing credentials, got nil")
	}

	// Case 2: API Error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Forbidden"))
	}))
	defer server.Close()

	notifier = NewDiscordBotNotifier("token", "channel")
	notifier.Client = &http.Client{
		Transport: &testTransport{
			TargetURL: server.URL,
		},
	}

	err = notifier.AddReaction(ctx, "msg_123", "check")
	if err == nil {
		t.Error("expected error for API failure, got nil")
	}
	if !strings.Contains(err.Error(), "discord api error: 403") {
		t.Errorf("expected 403 error, got %v", err)
	}
}

func TestDiscordNotifier_AddReaction_EmojiMapping(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// mapEmoji returns URL-encoded strings, but httptest.Server decodes r.URL.Path.
		// So we expect the decoded characters.
		{"white_check_mark", "✅"},
		{":white_check_mark:", "✅"},
		{"x", "❌"},
		{":x:", "❌"},
		{"warning", "⚠️"},
		{":warning:", "⚠️"},
		{"other", "other"}, // Default case
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify the URL path contains the expected mapped emoji
				// Path format: /api/v10/channels/%s/messages/%s/reactions/%s/@me
				expectedPathSuffix := "/reactions/" + tt.expected + "/@me"
				if !strings.HasSuffix(r.URL.Path, expectedPathSuffix) {
					t.Errorf("expected path suffix %q, got %q", expectedPathSuffix, r.URL.Path)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			notifier := NewDiscordBotNotifier("token", "channel")
			notifier.Client = &http.Client{
				Transport: &testTransport{
					TargetURL: server.URL,
				},
			}

			err := notifier.AddReaction(context.Background(), "msg_123", tt.input)
			if err != nil {
				t.Fatalf("AddReaction failed: %v", err)
			}
		})
	}
}

type testTransport struct {
	TargetURL string
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Helper to redirect to test server but preserve the original path
	// The original request is to https://discord.com/api/v10/channels/12345/messages
	// The test server url is http://127.0.0.1:xxx
	// We want http://127.0.0.1:xxx/api/v10/channels/12345/messages

	// 1. Create request to TargetURL (this gives us base scheme/host)
	targetReq, _ := http.NewRequest(req.Method, t.TargetURL, req.Body)

	// 2. Append original path
	targetReq.URL.Path = req.URL.Path

	// 3. Copy headers
	targetReq.Header = req.Header

	// 4. Send using default client (which handles test server local traffic)
	return http.DefaultClient.Do(targetReq)
}
