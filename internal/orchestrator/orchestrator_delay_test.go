package orchestrator

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestOrchestrator_RunAfter(t *testing.T) {
	poller := &MockPoller{}
	poller.On("Poll", mock.Anything, mock.Anything).Return([]WorkItem{}, nil)
	spawner := &MockSpawner{}
	spawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(poller, spawner, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	go orch.Run(ctx, logger)

	// Create an item with RunAfter 500ms in the future
	item := WorkItem{
		ID:       "delay-1",
		Summary:  "Delayed Job",
		RunAfter: time.Now().Add(500 * time.Millisecond),
	}

	err := orch.SubmitJob(ctx, item, nil)
	assert.NoError(t, err)

	// Should be pending immediately
	job, err := orch.GetJob("delay-1")
	assert.NoError(t, err)
	assert.Equal(t, "Pending", job.Status)
	assert.Equal(t, 0, orch.GetStatus().ActiveSpawns)

	// Wait 200ms, should still be pending
	time.Sleep(200 * time.Millisecond)
	job, _ = orch.GetJob("delay-1")
	assert.Equal(t, "Pending", job.Status)
	assert.Equal(t, 0, orch.GetStatus().ActiveSpawns)

	// Wait until enough time has elapsed
	assert.Eventually(t, func() bool {
		job, _ := orch.GetJob("delay-1")
		// The mock spawner returns nil for Spawn so it should transition to Completed
		return job.Status == "Completed" || job.Status == "Spawning"
	}, 2*time.Second, 100*time.Millisecond)
}
