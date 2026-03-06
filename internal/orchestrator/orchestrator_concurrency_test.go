package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestrator_MaxConcurrentJobs_LimitEnforced(t *testing.T) {
	// Setup
	item1 := WorkItem{ID: "JOB-1"}
	item2 := WorkItem{ID: "JOB-2"}

	poller := newMockPoller([]WorkItem{item1, item2})

	// Use a spawner that blocks so jobs stay active
	blockCh := make(chan struct{})
	spawner := &mockSpawner{blockCh: blockCh}

	// Max 1 job at a time
	orch := New(poller, spawner, 10*time.Millisecond)
	orch.MaxConcurrentJobs = 1

	ctx, cancel := context.WithCancel(context.Background())
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Act
	go orch.Run(ctx, logger)

	// Wait for poll
	time.Sleep(50 * time.Millisecond)

	// Assert
	status := orch.GetStatus()
	assert.Equal(t, 1, status.ActiveSpawns, "Should only spawn 1 job due to capacity limit")

	activeJobs := orch.GetActiveJobs()
	require.Len(t, activeJobs, 1)
	assert.Equal(t, "JOB-1", activeJobs[0].ID) // First job should be active

	// Cleanup
	close(blockCh)
	cancel()
}

func TestOrchestrator_MaxConcurrentJobs_ManualSubmit(t *testing.T) {
	// Setup
	poller := newMockPoller(nil)
	blockCh := make(chan struct{})
	spawner := &mockSpawner{blockCh: blockCh}

	orch := New(poller, spawner, 10*time.Millisecond)
	orch.MaxConcurrentJobs = 1

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Act
	err1 := orch.SubmitJob(ctx, WorkItem{ID: "MANUAL-1"}, logger)
	require.NoError(t, err1, "First job should be submitted successfully")

	err2 := orch.SubmitJob(ctx, WorkItem{ID: "MANUAL-2"}, logger)

	// Assert
	require.ErrorIs(t, err2, ErrAtCapacity, "Second job should fail due to capacity limit")

	// Cleanup
	close(blockCh)
}

func TestOrchestrator_MaxConcurrentJobs_Unlimited(t *testing.T) {
	// Setup
	item1 := WorkItem{ID: "JOB-1"}
	item2 := WorkItem{ID: "JOB-2"}

	poller := newMockPoller([]WorkItem{item1, item2})
	blockCh := make(chan struct{})
	spawner := &mockSpawner{blockCh: blockCh}

	// 0 means unlimited
	orch := New(poller, spawner, 10*time.Millisecond)
	orch.MaxConcurrentJobs = 0

	ctx, cancel := context.WithCancel(context.Background())
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Act
	go orch.Run(ctx, logger)

	// Wait for poll
	time.Sleep(50 * time.Millisecond)

	// Assert
	status := orch.GetStatus()
	assert.Equal(t, 2, status.ActiveSpawns, "Should spawn all jobs if unlimited")

	activeJobs := orch.GetActiveJobs()
	assert.Len(t, activeJobs, 2)

	// Cleanup
	close(blockCh)
	cancel()
}
