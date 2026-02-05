package notify

import (
	"context"
	"testing"
	"time"
)

func TestHandleEvents_NilClient(t *testing.T) {
	m := &Manager{
		socketClient: nil,
	}

	// This should return immediately and not panic or block
	// We can't verify it returned immediately easily without timeout,
	// but if it blocked, the test would hang.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan bool)
	go func() {
		m.HandleEvents(ctx)
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("HandleEvents did not return immediately for nil client")
	}
}
