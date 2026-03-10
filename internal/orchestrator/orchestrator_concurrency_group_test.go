package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestOrchestrator_ConcurrencyGroup(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	// Setup Spawner mocks
	// First job will be spawned, but simulate it takes a long time so it remains "active"
	mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-1"
	})).Return(nil).Run(func(args mock.Arguments) {
		time.Sleep(500 * time.Millisecond)
	})

	// Cancel will be called for JOB-1
	mockSpawner.On("Cancel", mock.Anything, "JOB-1").Return(nil)

	// Second job will be spawned and return immediately
	mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-2"
	})).Return(nil)

	// Submit Job 1
	err := orch.SubmitJob(ctx, WorkItem{
		ID:               "JOB-1",
		Summary:          "First job",
		ConcurrencyGroup: "group-A",
		CancelInProgress: true,
	}, logger)
	assert.NoError(t, err)

	// Verify Job 1 is active
	activeJobs := orch.GetActiveJobs()
	assert.Len(t, activeJobs, 1)
	assert.Equal(t, "JOB-1", activeJobs[0].ID)

	// Submit Job 2 in the same concurrency group
	err = orch.SubmitJob(ctx, WorkItem{
		ID:               "JOB-2",
		Summary:          "Second job",
		ConcurrencyGroup: "group-A",
		CancelInProgress: true,
	}, logger)
	assert.NoError(t, err)

	// Give a moment for the cancellation and background goroutines to run
	time.Sleep(50 * time.Millisecond)

	// We expect Job 1 to be cancelled (moved to completed queue eventually or cancelled state)
	// Because of mock delays, we just want to ensure Cancel was called
	mockSpawner.AssertCalled(t, "Cancel", mock.Anything, "JOB-1")
}
