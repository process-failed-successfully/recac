package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestrator_RetryJob(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond)

	ctx := context.Background()

	// 1. Submit a job
	item := WorkItem{ID: "RETRY-1", Summary: "Retry Me"}
	err := orch.SubmitJob(ctx, item, silentLogger)
	require.NoError(t, err)

	// Wait for job to complete.
	// Since spawner returns immediately, spawnWorker should finish quickly.
	// We can poll GetJob until it returns from history.
	require.Eventually(t, func() bool {
		job, err := orch.GetJob("RETRY-1")
		if err != nil {
			return false
		}
		return job.Status == "Completed"
	}, 1*time.Second, 10*time.Millisecond, "Job should complete")

	// Verify it is in history
	completed := orch.GetCompletedJobs()
	require.Len(t, completed, 1)
	assert.Equal(t, "RETRY-1", completed[0].ID)

	// Verify not active
	active := orch.GetActiveJobs()
	assert.Empty(t, active)

	// 2. Retry the job
	err = orch.RetryJob(ctx, "RETRY-1", silentLogger)
	require.NoError(t, err)

	// Verify it is active again
	active = orch.GetActiveJobs()
	require.Len(t, active, 1)
	assert.Equal(t, "RETRY-1", active[0].ID)
	assert.Equal(t, "Spawning", active[0].Status)

	// Wait for it to complete again
	require.Eventually(t, func() bool {
		return len(orch.GetActiveJobs()) == 0
	}, 1*time.Second, 10*time.Millisecond, "Job should complete again")

	// Verify history has 2 entries
	completed = orch.GetCompletedJobs()
	assert.Len(t, completed, 2)
	assert.Equal(t, "RETRY-1", completed[0].ID)
	assert.Equal(t, "RETRY-1", completed[1].ID)

	// 3. Test invalid retry (active)
	// Submit a new job and try to retry immediately
	item2 := WorkItem{ID: "RETRY-2", Summary: "Active"}
	// Use a blocking spawner to keep it active
	blockCh := make(chan struct{})
	spawner2 := &mockSpawner{blockCh: blockCh}
	orch2 := New(poller, spawner2, 50*time.Millisecond)

	err = orch2.SubmitJob(ctx, item2, silentLogger)
	require.NoError(t, err)

	// Should be active
	require.Len(t, orch2.GetActiveJobs(), 1)

	// Retry should fail
	err = orch2.RetryJob(ctx, "RETRY-2", silentLogger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already active")

	// Clean up
	close(blockCh)

	// 4. Test invalid retry (not found)
	err = orch.RetryJob(ctx, "NON-EXISTENT", silentLogger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
