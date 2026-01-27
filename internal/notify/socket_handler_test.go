package notify

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/stretchr/testify/assert"
)

func TestHandleEvents(t *testing.T) {
	// Mock Slack API Server
	apiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle chat.postMessage
		if strings.Contains(r.URL.Path, "chat.postMessage") {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"ok": true, "ts": "1234.5678"}`))
			return
		}
		// Handle other requests (like Ack which calls some endpoint? No, Ack sends via WS usually but here we are using HTTP client for REST calls if any)
		// socketmode.Client.Ack sends a request if it's not connected via WS?
		// Actually, socketmode uses WebSocket to receive, but Ack is sent back over WS or HTTP?
		// socketmode/Client.go: Ack() writes to websocket.

		// If we don't have a real websocket connection, Ack() might block or fail or do nothing if we didn't run the client loop.
		// socketmode.Client has a `send` channel internally. Run() loop consumes it.
		// Since we are NOT running `m.socketClient.RunContext(ctx)`, `Ack()` puts message on channel and it stays there.
		// It won't panic, but it won't send anything.
		// But HandleEvents calls Ack.

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok": true}`))
	})
	ts := httptest.NewServer(apiHandler)
	defer ts.Close()

	// Create Slack Client pointing to mock server
	api := slack.New("xoxb-test", slack.OptionAPIURL(ts.URL+"/"))

	// Create Socket Mode Client
	smClient := socketmode.New(api)

	// Create Manager
	// We want to capture logs
	var logs []string
	logger := func(format string, args ...interface{}) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}

	m := &Manager{
		logger:       logger,
		socketClient: smClient,
		client:       api,
	}

	// Run HandleEvents in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.HandleEvents(ctx)

	// Simulate events
	// 1. Connected
	smClient.Events <- socketmode.Event{
		Type: socketmode.EventTypeConnected,
	}

	// Wait a bit for processing
	time.Sleep(50 * time.Millisecond)
	assert.Contains(t, logs, "Connected to Slack Socket Mode via WebSocket!")

	// 2. Mention Event
	innerEvent := slackevents.EventsAPIEvent{
		Type: slackevents.CallbackEvent,
		InnerEvent: slackevents.EventsAPIInnerEvent{
			Data: &slackevents.AppMentionEvent{
				User:    "U123",
				Text:    "Hello bot",
				Channel: "C123",
			},
		},
	}

	// Create a dummy request
	req := &socketmode.Request{
		Type: "events_api",
	}

	evt := socketmode.Event{
		Type:    socketmode.EventTypeEventsAPI,
		Data:    innerEvent,
		Request: req,
	}

	smClient.Events <- evt

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Check logs
	found := false
	for _, l := range logs {
		if strings.Contains(l, "Received Mention: Hello bot") {
			found = true
			break
		}
	}
	assert.True(t, found, "Should have logged received mention")

	// 3. Connecting
	smClient.Events <- socketmode.Event{
		Type: socketmode.EventTypeConnecting,
	}
	time.Sleep(10 * time.Millisecond)
	assert.Contains(t, logs, "Connecting to Slack Socket Mode...")
}
