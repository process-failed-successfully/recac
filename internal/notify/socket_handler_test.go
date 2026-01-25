package notify

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slack-go/slack/socketmode"
)

func TestHandleEvents(t *testing.T) {
	// Setup Logger
	var logMu sync.Mutex
	var logs []string
	logger := func(format string, args ...interface{}) {
		logMu.Lock()
		defer logMu.Unlock()
		if len(args) > 0 {
			logs = append(logs, format) // Simplified for matching
		} else {
			logs = append(logs, format)
		}
	}

	// Create Manager with manual socket client
	m := &Manager{
		logger: logger,
	}

	// Create socket client
	// We can't easily instantiate a functional socketmode.Client without a valid slack.Client and connection.
	// But we can create a struct and inject the channel if we can access the struct fields?
	// socketmode.Client struct has exported Events field.
	// But `New` returns a pointer.
	// We can try to construct it manually if possible, or use New with nil client?
	// socketmode.New(nil) might panic.

	// Let's rely on the fact that socketmode.Client is a struct and Events is exported.
	client := socketmode.Client{
		Events: make(chan socketmode.Event, 10),
	}
	m.socketClient = &client

	// Start Handler
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		m.HandleEvents(ctx)
	}()

	// Test 1: Connecting
	client.Events <- socketmode.Event{Type: socketmode.EventTypeConnecting}

	// Wait a bit for processing
	time.Sleep(100 * time.Millisecond)

	logMu.Lock()
	foundConnecting := false
	for _, l := range logs {
		if strings.Contains(l, "Connecting to Slack") {
			foundConnecting = true
			break
		}
	}
	logMu.Unlock()

	if !foundConnecting {
		t.Error("Expected log 'Connecting to Slack Socket Mode...'")
	}

	// Test 2: Connected
	client.Events <- socketmode.Event{Type: socketmode.EventTypeConnected}
	time.Sleep(100 * time.Millisecond)

	logMu.Lock()
	foundConnected := false
	for _, l := range logs {
		if strings.Contains(l, "Connected to Slack") {
			foundConnected = true
			break
		}
	}
	logMu.Unlock()

	if !foundConnected {
		t.Error("Expected log 'Connected to Slack Socket Mode...'")
	}

	// Stop
	cancel()
	wg.Wait()
}
