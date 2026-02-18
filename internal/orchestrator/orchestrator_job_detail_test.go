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

func TestOrchestrator_GetJob(t *testing.T) {
	// Setup
	poller := newMockPoller(nil)
	spawner := &mockSpawner{
		blockCh: make(chan struct{}), // Block spawning to keep job active
	}
	orch := New(poller, spawner, 10*time.Millisecond)

	// Manual item with full details
	item := WorkItem{
		ID:          "JOB-DETAIL-1",
		Summary:     "Test Job Details",
		Description: "A detailed description",
		RepoURL:     "https://github.com/example/repo",
		EnvVars:     map[string]string{"KEY": "VALUE"},
	}

	// Submit
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := orch.SubmitJob(ctx, item, logger)
	require.NoError(t, err)

	// Act: GetJob
	jobInfo, err := orch.GetJob("JOB-DETAIL-1")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, item.ID, jobInfo.ID)
	assert.Equal(t, item.Summary, jobInfo.Summary)
	assert.Equal(t, item.Description, jobInfo.WorkItem.Description)
	assert.Equal(t, item.RepoURL, jobInfo.WorkItem.RepoURL)
	assert.Equal(t, item.EnvVars, jobInfo.WorkItem.EnvVars)

	// Act: Get non-existent job
	_, err = orch.GetJob("NON-EXISTENT")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Cleanup: unblock spawner to let goroutine finish
	close(spawner.blockCh)
}

func TestOrchestrator_Run_PopulatesWorkItem(t *testing.T) {
	// Setup
	item := WorkItem{
		ID:          "POLL-JOB-1",
		Summary:     "Polled Job",
		Description: "Polled Description",
		RepoURL:     "https://github.com/example/polled-repo",
	}
	poller := newMockPoller([]WorkItem{item})

	// Use a spawner that blocks so we can inspect active jobs
	blockCh := make(chan struct{})
	spawner := &mockSpawner{blockCh: blockCh}

	orch := New(poller, spawner, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Run in background
	go func() {
		orch.Run(ctx, logger)
	}()

	// Wait a bit for poll to happen and spawn to start (and block)
	time.Sleep(50 * time.Millisecond)

	// Check Job Info
	jobInfo, err := orch.GetJob("POLL-JOB-1")
	require.NoError(t, err)
	assert.Equal(t, item.ID, jobInfo.ID)
	assert.Equal(t, item.Description, jobInfo.WorkItem.Description)
	assert.Equal(t, item.RepoURL, jobInfo.WorkItem.RepoURL)

	// Cleanup
	close(blockCh) // Unblock spawner
	cancel()       // Stop orchestrator
}
