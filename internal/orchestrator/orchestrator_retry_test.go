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

func TestOrchestrator_RetryJob(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Retry active job fails", func(t *testing.T) {
		poller := &mockPoller{}
		spawner := &mockSpawner{}
		orch := New(poller, spawner, time.Minute)

		// Manually add an active job
		orch.activeJobs["JOB-1"] = JobInfo{ID: "JOB-1"}

		err := orch.RetryJob(context.Background(), "JOB-1", logger)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "currently active")
	})

	t.Run("Retry non-existent job fails", func(t *testing.T) {
		poller := &mockPoller{}
		spawner := &mockSpawner{}
		orch := New(poller, spawner, time.Minute)

		err := orch.RetryJob(context.Background(), "JOB-999", logger)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found in history")
	})

	t.Run("Retry completed job succeeds", func(t *testing.T) {
		poller := &mockPoller{}
		spawner := &mockSpawner{}
		orch := New(poller, spawner, time.Minute)

		// Add a completed job to history
		item := WorkItem{ID: "JOB-COMPLETED", Summary: "Completed Job"}
		orch.completedJobs = append(orch.completedJobs, JobInfo{
			ID:       "JOB-COMPLETED",
			Status:   "Completed",
			WorkItem: item,
		})

		err := orch.RetryJob(context.Background(), "JOB-COMPLETED", logger)
		require.NoError(t, err)

		// Verify it's active
		orch.mu.RLock()
		_, active := orch.activeJobs["JOB-COMPLETED"]
		orch.mu.RUnlock()
		assert.True(t, active)

		// Allow mock spawner to finish (it's immediate in mockSpawner unless blocked)
		time.Sleep(10 * time.Millisecond)

		// Check spawner called
		spawner.mu.Lock()
		assert.Equal(t, 1, len(spawner.spawned))
		assert.Equal(t, "JOB-COMPLETED", spawner.spawned[0].ID)
		spawner.mu.Unlock()
	})

	t.Run("Retry failed job succeeds", func(t *testing.T) {
		poller := &mockPoller{}
		spawner := &mockSpawner{}
		orch := New(poller, spawner, time.Minute)

		// Add a failed job to history
		item := WorkItem{ID: "JOB-FAILED", Summary: "Failed Job"}
		orch.completedJobs = append(orch.completedJobs, JobInfo{
			ID:       "JOB-FAILED",
			Status:   "Failed",
			WorkItem: item,
		})

		err := orch.RetryJob(context.Background(), "JOB-FAILED", logger)
		require.NoError(t, err)

		// Verify it's active
		orch.mu.RLock()
		_, active := orch.activeJobs["JOB-FAILED"]
		orch.mu.RUnlock()
		assert.True(t, active)
	})
}
