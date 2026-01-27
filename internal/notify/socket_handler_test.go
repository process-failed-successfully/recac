package notify

import (
	"context"
	"testing"
	"time"

	"github.com/slack-go/slack/socketmode"
)

func TestManager_HandleEvents(t *testing.T) {
	// 1. Create a Manager with a manually constructed socketClient
	// socketmode.Client is a struct. We can't easily mock its internal behavior,
	// but HandleEvents reads from its public Events channel.

	// Create a buffered channel to simulate events
	eventsCh := make(chan socketmode.Event, 1)

	client := &socketmode.Client{
		Events: eventsCh,
	}

	m := &Manager{
		socketClient: client,
		logger: func(format string, args ...interface{}) {
			// t.Logf(format, args...)
		},
	}

	// 2. Start HandleEvents in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan bool)
	go func() {
		m.HandleEvents(ctx)
		close(done)
	}()

	// 3. Send a dummy event
	eventsCh <- socketmode.Event{
		Type: socketmode.EventTypeConnected,
	}

	// 4. Wait a bit to ensure it processes
	time.Sleep(100 * time.Millisecond)

	// 5. Cancel context and wait for exit
	cancel()
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("HandleEvents did not exit")
	}
}
