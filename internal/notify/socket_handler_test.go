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

// MockSocketClient implements SocketClient interface for testing
type MockSocketClient struct {
	AckFunc func(req socketmode.Request, payload ...interface{})
}

func (m *MockSocketClient) Ack(req socketmode.Request, payload ...interface{}) {
	if m.AckFunc != nil {
		m.AckFunc(req, payload...)
	}
}

func TestManager_HandleEvents(t *testing.T) {
	// 1. Create Manager with mock poster
	var mu sync.Mutex
	capturedMessages := []string{}

	mockPoster := &mockSlackPoster{
		postMessageFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
			mu.Lock()
			capturedMessages = append(capturedMessages, channelID)
			mu.Unlock()
			return "ts", "channel", nil
		},
	}

	capturedLogs := []string{}
	logger := func(format string, args ...interface{}) {
		mu.Lock()
		defer mu.Unlock()
		capturedLogs = append(capturedLogs, fmt.Sprintf(format, args...))
	}

	m := &Manager{
		client:       mockPoster,
		// No socketClient needed because we pass mock client to HandleEvents
		logger:       logger,
	}

	// 2. Mock Socket Client
	ackCalled := false
	mockClient := &MockSocketClient{
		AckFunc: func(req socketmode.Request, payload ...interface{}) {
			mu.Lock()
			ackCalled = true
			mu.Unlock()
		},
	}

	// 3. Create Events Channel
	events := make(chan socketmode.Event, 5)

	// 4. Run HandleEvents in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.HandleEvents(ctx, events, mockClient)

	// 5. Send Events

	// Event: Connecting
	events <- socketmode.Event{
		Type: socketmode.EventTypeConnecting,
	}
	time.Sleep(20 * time.Millisecond) // wait for processing

	// Event: Connected
	events <- socketmode.Event{
		Type: socketmode.EventTypeConnected,
	}
	time.Sleep(20 * time.Millisecond)

	// Event: App Mention
	appMention := &slackevents.AppMentionEvent{
		Type:    "app_mention",
		User:    "U123456",
		Text:    "Hello Bot",
		Channel: "C123456",
	}

	innerEvent := slackevents.EventsAPIInnerEvent{
		Type: "app_mention",
		Data: appMention,
	}

	eventsAPIEvent := slackevents.EventsAPIEvent{
		Type:       slackevents.CallbackEvent,
		InnerEvent: innerEvent,
	}

	req := &socketmode.Request{
		Type: "events_api",
		EnvelopeID: "envelope_123",
	}

	events <- socketmode.Event{
		Type:    socketmode.EventTypeEventsAPI,
		Data:    eventsAPIEvent,
		Request: req,
	}

	// Allow time for processing
	time.Sleep(100 * time.Millisecond)

	// 6. Verification
	mu.Lock()
	logs := ""
	for _, l := range capturedLogs {
		logs += l + "\n"
	}
	isAckCalled := ackCalled
	isPostMessageCalled := len(capturedMessages) > 0
	mu.Unlock()

	assert.Contains(t, logs, "Connecting to Slack Socket Mode")
	assert.Contains(t, logs, "Connected to Slack Socket Mode")
	assert.Contains(t, logs, "Received Mention: Hello Bot")

	assert.True(t, isAckCalled, "Ack should have been called")
	assert.True(t, isPostMessageCalled, "PostMessage should have been called")
	if isPostMessageCalled {
		mu.Lock()
		assert.Equal(t, "C123456", capturedMessages[0])
		mu.Unlock()
	}
}

func TestManager_HandleEvents_ConnectionError(t *testing.T) {
	var mu sync.Mutex
	capturedLogs := []string{}
	logger := func(format string, args ...interface{}) {
		mu.Lock()
		defer mu.Unlock()
		capturedLogs = append(capturedLogs, fmt.Sprintf(format, args...))
	}

	m := &Manager{
		logger: logger,
	}

	mockClient := &MockSocketClient{} // No Ack expected

	events := make(chan socketmode.Event, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.HandleEvents(ctx, events, mockClient)

	events <- socketmode.Event{
		Type: socketmode.EventTypeConnectionError,
	}
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	logs := ""
	for _, l := range capturedLogs {
		logs += l + "\n"
	}
	mu.Unlock()

	assert.Contains(t, logs, "Connection failed. Retrying later...")
}
