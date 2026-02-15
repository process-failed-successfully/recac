package notify

import (
	"context"
	"testing"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
	"github.com/stretchr/testify/assert"
)

func TestHandleEvents(t *testing.T) {
	// Create a dummy Slack client
	api := slack.New("dummy-token")
	client := socketmode.New(api)

	logChan := make(chan string, 10)
	logger := func(format string, args ...interface{}) {
		logChan <- format
	}

	m := &Manager{
		socketClient: client,
		logger:       logger,
		// mock client for PostMessage
		client: &mockSlackPoster{
			postMessageFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
				return "", "", nil
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.HandleEvents(ctx)

	// 1. Send Connecting Event
	client.Events <- socketmode.Event{
		Type: socketmode.EventTypeConnecting,
	}

	select {
	case log := <-logChan:
		assert.Contains(t, log, "Connecting to Slack Socket Mode")
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for connecting log")
	}

	// 2. Send Connection Error Event
	client.Events <- socketmode.Event{
		Type: socketmode.EventTypeConnectionError,
	}

	select {
	case log := <-logChan:
		assert.Contains(t, log, "Connection failed")
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for connection error log")
	}

	// 3. Send Connected Event
	client.Events <- socketmode.Event{
		Type: socketmode.EventTypeConnected,
	}

	select {
	case log := <-logChan:
		assert.Contains(t, log, "Connected to Slack Socket Mode")
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for connected log")
	}
}
