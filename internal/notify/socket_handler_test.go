package notify

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/stretchr/testify/assert"
)

func TestHandleEventsLoop(t *testing.T) {
	// Setup
	events := make(chan socketmode.Event)

	var mu sync.Mutex
	ackCalled := false
	var logs []string

	ackFunc := func(req socketmode.Request) {
		mu.Lock()
		defer mu.Unlock()
		ackCalled = true
	}

	logger := func(msg string, args ...interface{}) {
		mu.Lock()
		defer mu.Unlock()
		logs = append(logs, fmt.Sprintf(msg, args...))
	}

	m := &Manager{
		logger: logger,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start loop in background
	go m.handleEventsLoop(ctx, events, ackFunc)

	// Test Connecting
	events <- socketmode.Event{Type: socketmode.EventTypeConnecting}
	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, l := range logs {
			if l == "Connecting to Slack Socket Mode..." {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)

	// Test Connected
	events <- socketmode.Event{Type: socketmode.EventTypeConnected}
	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, l := range logs {
			if l == "Connected to Slack Socket Mode via WebSocket!" {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)

	// Test Connection Error
	events <- socketmode.Event{Type: socketmode.EventTypeConnectionError}
	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, l := range logs {
			if l == "Connection failed. Retrying later..." {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)

	// Test EventsAPI (App Mention)
	req := &socketmode.Request{}
	events <- socketmode.Event{
		Type:    socketmode.EventTypeEventsAPI,
		Request: req,
		Data: slackevents.EventsAPIEvent{
			Type: slackevents.CallbackEvent,
			InnerEvent: slackevents.EventsAPIInnerEvent{
				Data: &slackevents.AppMentionEvent{
					Text:    "Hello",
					Channel: "C123",
				},
			},
		},
	}
	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		if !ackCalled {
			return false
		}
		for _, l := range logs {
			if l == "Received Mention: Hello" {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)
}

func TestHandleEventsLoop_PostMessage(t *testing.T) {
	events := make(chan socketmode.Event)
	ackFunc := func(req socketmode.Request) {}

	var mu sync.Mutex
	postMessageCalled := false

	mockSlack := &mockSlackPoster{
		postMessageFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
			mu.Lock()
			defer mu.Unlock()
			postMessageCalled = true
			assert.Equal(t, "C123", channelID)
			return "", "", nil
		},
	}

	m := &Manager{
		client: mockSlack,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.handleEventsLoop(ctx, events, ackFunc)

	events <- socketmode.Event{
		Type:    socketmode.EventTypeEventsAPI,
		Request: &socketmode.Request{},
		Data: slackevents.EventsAPIEvent{
			Type: slackevents.CallbackEvent,
			InnerEvent: slackevents.EventsAPIInnerEvent{
				Data: &slackevents.AppMentionEvent{
					Text:    "Hello",
					Channel: "C123",
				},
			},
		},
	}

	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return postMessageCalled
	}, time.Second, 10*time.Millisecond)
}
