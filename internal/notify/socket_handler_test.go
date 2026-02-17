package notify

import (
	"context"
	"testing"
	"time"

	"github.com/slack-go/slack/socketmode"
)

func TestHandleEvents_Cancel(t *testing.T) {
	m := &Manager{
		socketClient: &socketmode.Client{
			Events: make(chan socketmode.Event),
		},
		logger: func(format string, args ...interface{}) {},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	done := make(chan bool)
	go func() {
		m.HandleEvents(ctx)
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("HandleEvents did not return on cancelled context")
	}
}

func TestHandleEvents_Events(t *testing.T) {
	events := make(chan socketmode.Event, 1)
	m := &Manager{
		socketClient: &socketmode.Client{
			Events: events,
		},
		logger: func(format string, args ...interface{}) {},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Send an event
	events <- socketmode.Event{
		Type: socketmode.EventTypeConnected,
	}

	// Run in background
	go func() {
		m.HandleEvents(ctx)
	}()

	// Wait a bit to ensure it processed (hard to verify without side effects)
	time.Sleep(10 * time.Millisecond)
}
