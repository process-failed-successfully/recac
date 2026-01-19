package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSlackNotifier_Notify(t *testing.T) {
	receivedMessage := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST request, got %s", r.Method)
		}

		var payload map[string]string
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &payload)
		receivedMessage = payload["text"]

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewSlackNotifier(server.URL)
	ctx := context.Background()
	message := "Task completed successfully!"

	err := notifier.Notify(ctx, message)
	if err != nil {
		t.Fatalf("Notify failed: %v", err)
	}

	if receivedMessage != message {
		t.Errorf("expected message %q, got %q", message, receivedMessage)
	}
}

func TestSlackNotifier_Notify_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	notifier := NewSlackNotifier(server.URL)
	ctx := context.Background()

	err := notifier.Notify(ctx, "test")
	if err == nil {
		t.Error("expected error for non-OK status code, got nil")
	}
}

func TestSlackNotifier_Notify_EdgeCases(t *testing.T) {
	t.Run("empty webhook url", func(t *testing.T) {
		notifier := NewSlackNotifier("")
		ctx := context.Background()
		err := notifier.Notify(ctx, "test")
		if err == nil {
			t.Error("expected error for empty webhook url, got nil")
		}
	})

	t.Run("invalid url", func(t *testing.T) {
		// Control character in URL to force http.NewRequest error
		notifier := NewSlackNotifier("http://example.com/foo\x7f")
		ctx := context.Background()
		err := notifier.Notify(ctx, "test")
		if err == nil {
			t.Error("expected error for invalid url, got nil")
		}
	})

	t.Run("network error", func(t *testing.T) {
		// Create a server and close it immediately to force connection error
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		server.Close()

		notifier := NewSlackNotifier(server.URL)
		ctx := context.Background()
		err := notifier.Notify(ctx, "test")
		if err == nil {
			t.Error("expected error for closed server, got nil")
		}
	})
}
