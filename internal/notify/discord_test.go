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

	// Intercept URL construction in SendBotMessage?
	// The implementation hardcodes "https://discord.com/api/v10/...".
	// We cannot easily test this without mocking http.Client or overriding URL.
	// However, we can use a custom Transport in the client that redirects to our test server?
	// But the URL in request is absolute.

	// We'll skip deep mocking here and assume unit logic is sound if Webhook works,
	// unless we refactor to allow BaseURL injection.
	// For "parity" speed, I will refactor `discord.go` to allow BaseURL override?
	// Or just trust it.
	// Given "security focused", I should verify.
	// But `internal/notify/discord.go` uses hardcoded URL.

	// I will just test the struct logic, not the actual network call to discord.com unless I inject a custom client with Transport that hijacks requests.

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
