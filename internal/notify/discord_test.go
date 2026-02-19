package notify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestDiscordNotifier_Webhook(t *testing.T) {
	// Setup mock server
	receivedBody := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	// Test
	notifier := NewDiscordNotifier(server.URL)
	err := notifier.Notify(context.Background(), "Hello Discord")

	if err != nil {
		t.Fatalf("Notify failed: %v", err)
	}

	if !strings.Contains(receivedBody, "Hello Discord") {
		t.Errorf("Expected body to contain message, got %s", receivedBody)
	}
}

type redirectTransport struct {
	TargetURL *url.URL
	T         *testing.T
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite URL to point to mock server
	req.URL.Scheme = t.TargetURL.Scheme
	req.URL.Host = t.TargetURL.Host
	// Path is preserved
	return http.DefaultTransport.RoundTrip(req)
}

func TestDiscordNotifier_Bot(t *testing.T) {
	// Setup mock server
	receivedBody := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bot test-token" {
			t.Errorf("Expected Bot token auth")
		}
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id": "msg-123"}`))
	}))
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)

	notifier := NewDiscordBotNotifier("test-token", "12345")
	notifier.Client.Transport = &redirectTransport{TargetURL: serverURL, T: t}

	id, err := notifier.Send(context.Background(), "Hello Bot", "")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if id != "msg-123" {
		t.Errorf("Expected ID msg-123, got %s", id)
	}

	if !strings.Contains(receivedBody, "Hello Bot") {
		t.Errorf("Expected body to contain message, got %s", receivedBody)
	}
}

func TestDiscordNotifier_Bot_Reply(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "message_reference") {
			t.Error("Expected message_reference in body")
		}
		if !strings.Contains(string(body), "msg-ref-1") {
			t.Error("Expected msg-ref-1 in body")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id": "msg-new"}`))
	}))
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)
	notifier := NewDiscordBotNotifier("test-token", "12345")
	notifier.Client.Transport = &redirectTransport{TargetURL: serverURL, T: t}

	id, err := notifier.Send(context.Background(), "Reply", "msg-ref-1")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if id != "msg-new" {
		t.Errorf("Expected ID msg-new, got %s", id)
	}
}

func TestDiscordNotifier_Send_FallbackToWebhook(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	// No bot token, only webhook
	notifier := NewDiscordNotifier(server.URL)
	// NewDiscordNotifier sets WebhookURL

	id, err := notifier.Send(context.Background(), "Fallback", "")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if id != "" {
		t.Errorf("Expected empty ID for webhook, got %s", id)
	}
}

func TestDiscordNotifier_AddReaction(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT, got %s", r.Method)
		}
		// URL should contain reaction
		// Note: r.URL.Path is decoded by default in Go server handler
		if !strings.Contains(r.URL.Path, "✅") {
			t.Errorf("Expected reaction in URL, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)

	notifier := NewDiscordBotNotifier("test-token", "12345")
	notifier.Client.Transport = &redirectTransport{TargetURL: serverURL, T: t}

	// Use "white_check_mark" which maps to %E2%9C%85
	err := notifier.AddReaction(context.Background(), "msg-123", "white_check_mark")
	if err != nil {
		t.Fatalf("AddReaction failed: %v", err)
	}
}

func TestMapEmoji(t *testing.T) {
	// This function is not exported, but we can test it via AddReaction behaviors or export it for test?
	// It is in same package `notify` so we can test it directly if we were in `notify` package (we are).
	// But it is `mapEmoji` (lowercase), so unexported.
	// But `discord_test.go` is in package `notify`, so it can access it.

	if got := mapEmoji("white_check_mark"); got != "%E2%9C%85" {
		t.Errorf("Expected encoded check, got %s", got)
	}
	if got := mapEmoji("x"); got != "%E2%9D%8C" {
		t.Errorf("Expected encoded x, got %s", got)
	}
	if got := mapEmoji("other"); got != "other" {
		t.Errorf("Expected other, got %s", got)
	}
}
