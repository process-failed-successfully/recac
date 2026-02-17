package notify

import (
	"context"
	"testing"
)

func TestManager_HandleEvents_NilSocket(t *testing.T) {
	m := NewManager(nil)
	// socketClient is nil by default unless configured
	ctx := context.Background()
	// Should return immediately
	m.HandleEvents(ctx)
}
