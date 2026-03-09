package orchestrator

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOrchestrator_GetStatus_Initial(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	interval := 1 * time.Minute
	orch := New(poller, spawner, interval)

	status := orch.GetStatus()

	assert.Equal(t, interval.String(), status.PollInterval)
	assert.Equal(t, "Not started", status.Uptime)
	assert.True(t, status.LastPoll.IsZero())
	assert.Equal(t, 0, status.LastPollItems)
	assert.Equal(t, 0, status.ActiveSpawns)
	assert.Equal(t, 0, status.TotalSpawns)
}

func TestOrchestrator_GetStatus_Running(t *testing.T) {
	poller := newMockPoller([]WorkItem{{ID: "TEST-1"}})
	spawner := &mockSpawner{}
	interval := 10 * time.Millisecond
	orch := New(poller, spawner, interval)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start orchestrator in a goroutine
	done := make(chan error)
	go func() {
		done <- orch.Run(ctx, silentLogger)
	}()

	// Wait for at least one poll
	time.Sleep(50 * time.Millisecond)

	status := orch.GetStatus()

	// Verify status updates
	assert.NotEqual(t, "Not started", status.Uptime)
	assert.False(t, status.LastPoll.IsZero())

	// TotalSpawns should be 1
	assert.Equal(t, 1, status.TotalSpawns)

	cancel()
	<-done
}

// blockingSpawner is a helper for testing active spawns
type blockingSpawner struct {
	blockCh chan struct{}
}

func (b *blockingSpawner) Spawn(ctx context.Context, item WorkItem) error {
	<-b.blockCh
	return nil
}
func (b *blockingSpawner) Cleanup(ctx context.Context, item WorkItem) error { return nil }
func (b *blockingSpawner) Cancel(ctx context.Context, jobID string) error { return nil }
func (b *blockingSpawner) Ping(ctx context.Context) error { return nil }
func (b *blockingSpawner) GetLogs(ctx context.Context, jobID string) (io.ReadCloser, error) { return nil, nil }

func TestOrchestrator_GetStatus_BlockingSpawn(t *testing.T) {
	poller := newMockPoller([]WorkItem{{ID: "TEST-1"}})
	blockCh := make(chan struct{})
	spawner := &blockingSpawner{blockCh: blockCh}

	// Use 100ms interval, wait 50ms so we catch it during the first interval
	orch := New(poller, spawner, 100*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run in background
	go func() {
		orch.Run(ctx, silentLogger)
	}()

	// Wait for poll to happen and spawn to start (blocked)
	time.Sleep(50 * time.Millisecond)

	status := orch.GetStatus()
	assert.Equal(t, 1, status.ActiveSpawns, "ActiveSpawns should be 1 while blocked")
	assert.Equal(t, 1, status.TotalSpawns, "TotalSpawns should be 1")
	assert.Equal(t, 1, status.LastPollItems, "LastPollItems should be 1")

	// Unblock
	close(blockCh)

	// Wait for spawn to finish
	time.Sleep(50 * time.Millisecond)

	status = orch.GetStatus()
	assert.Equal(t, 0, status.ActiveSpawns, "ActiveSpawns should be 0 after completion")
	assert.Equal(t, 1, status.TotalSpawns, "TotalSpawns should remain 1")
}
