package notify

import (
	"context"
	"testing"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSlackPoster for testing
type MockSlackPoster struct {
	mock.Mock
}

func (m *MockSlackPoster) PostMessageContext(ctx context.Context, channelID string, options ...slack.MsgOption) (string, string, error) {
	args := m.Called(ctx, channelID, options)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *MockSlackPoster) PostMessage(channelID string, options ...slack.MsgOption) (string, string, error) {
	// We need to match options carefully or just use anything
	// Note: testify mock handles variadic args by collapsing them into a slice if mocked as variadic?
	// Or we call .Called(channelID, options) where options is []MsgOption.
	// Let's assume passed as separate args in .Called? No, usually slice.
	args := m.Called(channelID, options)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *MockSlackPoster) AddReactionContext(ctx context.Context, name string, item slack.ItemRef) error {
	args := m.Called(ctx, name, item)
	return args.Error(0)
}

func TestHandleEvents(t *testing.T) {
	// Setup
	eventsChan := make(chan socketmode.Event, 10)
	ackCalls := 0
	ackFunc := func(req socketmode.Request, payload ...interface{}) {
		ackCalls++
	}

	mockPoster := new(MockSlackPoster)
	manager := &Manager{
		socketEvents: eventsChan,
		socketAck:    ackFunc,
		client:       mockPoster,
		logger:       func(format string, v ...interface{}) {}, // No-op logger
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run HandleEvents in background
	go manager.HandleEvents(ctx)

	// Test 1: Connecting (should just log)
	eventsChan <- socketmode.Event{Type: socketmode.EventTypeConnecting}

	// Test 2: App Mention
	req := &socketmode.Request{}
	innerEvent := &slackevents.AppMentionEvent{
		User:    "U123",
		Text:    "Hello bot",
		Channel: "C123",
	}
	eventsAPIEvent := slackevents.EventsAPIEvent{
		Type: slackevents.CallbackEvent,
		InnerEvent: slackevents.EventsAPIInnerEvent{
			Type: "app_mention",
			Data: innerEvent,
		},
	}

	// Expect PostMessage call
	// Note: variadic arguments in mock expectations can be tricky.
	// We expect PostMessage("C123", options...)
	// The implementation calls PostMessage(ev.Channel, slack.MsgOptionText(...))
	// So it passes 1 option.
	mockPoster.On("PostMessage", "C123", mock.Anything).Return("ts", "channel", nil)

	eventsChan <- socketmode.Event{
		Type:    socketmode.EventTypeEventsAPI,
		Data:    eventsAPIEvent,
		Request: req,
	}

	// Wait a bit for processing
	time.Sleep(100 * time.Millisecond)

	// Verify Ack called
	assert.Equal(t, 1, ackCalls)

	// Verify PostMessage called
	mockPoster.AssertExpectations(t)

	// Test 3: Connection Error (log only)
	eventsChan <- socketmode.Event{Type: socketmode.EventTypeConnectionError}

	// Test 4: Connected (log only)
	eventsChan <- socketmode.Event{Type: socketmode.EventTypeConnected}

	// Test 5: Invalid Event Data (should skip)
	eventsChan <- socketmode.Event{
		Type: socketmode.EventTypeEventsAPI,
		Data: "invalid data", // Not EventsAPIEvent
	}

	// Test 6: Unhandled Inner Event (should skip)
	eventsChan <- socketmode.Event{
		Type: socketmode.EventTypeEventsAPI,
		Data: slackevents.EventsAPIEvent{
			Type: "unknown_type",
		},
		Request: req,
	}

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Ack calls should increase for Test 6 (it acks before switch)
	assert.Equal(t, 2, ackCalls)
}
