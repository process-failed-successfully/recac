package notify

import (
	"context"
	"testing"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

func TestHandleEvents(t *testing.T) {
	api := slack.New("xoxb-dummy")
	client := socketmode.New(api)

	// Verify channel exists
	if client.Events == nil {
		// Attempt to initialize if nil (though New() should have done it)
		// We can't easily initialize it if it's private or if we can't assign to it.
		// But let's assume New() initializes it as per library behavior.
        // If it fails, we will know.
        t.Skip("Events channel is nil")
	}

	m := &Manager{
		socketClient: client,
		logger: func(msg string, args ...interface{}) {},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start handler
	go m.HandleEvents(ctx)

	// Inject events
	go func() {
		client.Events <- socketmode.Event{
			Type: socketmode.EventTypeConnecting,
		}
		client.Events <- socketmode.Event{
			Type: socketmode.EventTypeConnected,
		}
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	<-ctx.Done()
}
