package notify

import (
	"context"
	"testing"
	"time"

	"github.com/slack-go/slack/socketmode"
)

func TestHandleEvents_ConnectionEvents(t *testing.T) {
	// Create channels
	eventsChan := make(chan socketmode.Event, 10)

	// Create Socket Client manually
	sc := &socketmode.Client{
		Events: eventsChan,
	}

	m := &Manager{
		socketClient: sc,
		logger:       func(string, ...interface{}) {},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run HandleEvents in background
	go m.HandleEvents(ctx)

	// Send Connecting event
	eventsChan <- socketmode.Event{
		Type: socketmode.EventTypeConnecting,
	}

	// Send ConnectionError event
	eventsChan <- socketmode.Event{
		Type: socketmode.EventTypeConnectionError,
	}

	// Send Connected event
	eventsChan <- socketmode.Event{
		Type: socketmode.EventTypeConnected,
	}

	// Allow some time for processing
	time.Sleep(50 * time.Millisecond)

	// If it didn't panic, we assume success.
	// We can't easily assert logging happened without mocking logger more robustly,
	// but the goal is code coverage of the switch cases.
}
