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

func TestSlackNotifier_Notify_MissingURL(t *testing.T) {
	notifier := NewSlackNotifier("")
	ctx := context.Background()

	err := notifier.Notify(ctx, "test")
	if err == nil {
		t.Error("expected error for missing webhook URL, got nil")
	}
}

func TestSlackNotifier_Notify_ClientError(t *testing.T) {
	// Using a dummy URL and closed server to simulate connection refused (client error)
	// But clearer way is to use a client that always errors
	notifier := NewSlackNotifier("http://invalid-url")
	notifier.Client = &http.Client{
		Transport: &errorTransport{},
	}

	ctx := context.Background()
	err := notifier.Notify(ctx, "test")
	if err == nil {
		t.Error("expected error for client failure, got nil")
	}
}

func TestSlackNotifier_Notify_RequestError(t *testing.T) {
	// Invalid URL to trigger NewRequest error
	notifier := NewSlackNotifier("http://invalid\nurl")
	ctx := context.Background()

	err := notifier.Notify(ctx, "test")
	if err == nil {
		t.Error("expected error for invalid URL, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create request") {
		t.Errorf("Expected 'failed to create request', got %v", err)
	}
}

type errorTransport struct{}

func (t *errorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, io.ErrUnexpectedEOF
}
