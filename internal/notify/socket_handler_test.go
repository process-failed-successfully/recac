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

func TestHandleEvents_NilClient(t *testing.T) {
	m := &Manager{}
	// Should return immediately without panic
	m.HandleEvents(context.Background())
}

func TestHandleEvents_ContextDone(t *testing.T) {
	client := socketmode.New(slack.New("token"))
	m := &Manager{socketClient: client}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	m.HandleEvents(ctx)
	// If it doesn't block forever, it passes
}

func TestHandleEvents_Events(t *testing.T) {
	client := socketmode.New(slack.New("token"))

	var (
		mu                sync.Mutex
		logs              []string
		postMessageCalled bool
	)

	logger := func(format string, args ...interface{}) {
		mu.Lock()
		defer mu.Unlock()
		logs = append(logs, fmt.Sprintf(format, args...))
	}

	mockSlack := &mockSlackPoster{
		postMessageFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
			mu.Lock()
			defer mu.Unlock()
			postMessageCalled = true
			assert.Equal(t, "channel1", channelID)
			return "", "", nil
		},
	}

	m := &Manager{
		socketClient: client,
		logger:       logger,
		client:       mockSlack,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.HandleEvents(ctx)

	// Send Events
	client.Events <- socketmode.Event{Type: socketmode.EventTypeConnecting}
	client.Events <- socketmode.Event{Type: socketmode.EventTypeConnectionError}
	client.Events <- socketmode.Event{Type: socketmode.EventTypeConnected}

	// Invalid EventsAPIEvent
	client.Events <- socketmode.Event{Type: socketmode.EventTypeEventsAPI, Data: "invalid"}

	// Valid EventsAPIEvent -> Mention
	req := &socketmode.Request{}
	eventsAPIEvent := slackevents.EventsAPIEvent{
		Type: slackevents.CallbackEvent,
		InnerEvent: slackevents.EventsAPIInnerEvent{
			Data: &slackevents.AppMentionEvent{
				Text:    "hello",
				Channel: "channel1",
			},
		},
	}
	client.Events <- socketmode.Event{
		Type:    socketmode.EventTypeEventsAPI,
		Data:    eventsAPIEvent,
		Request: req,
	}

	// Give it a moment to process
	time.Sleep(100 * time.Millisecond)
	cancel()

	mu.Lock()
	defer mu.Unlock()
	assert.Contains(t, logs, "Connecting to Slack Socket Mode...")
	assert.Contains(t, logs, "Connection failed. Retrying later...")
	assert.Contains(t, logs, "Connected to Slack Socket Mode via WebSocket!")
	assert.Contains(t, logs, "Received Mention: hello")
	assert.True(t, postMessageCalled)
}
