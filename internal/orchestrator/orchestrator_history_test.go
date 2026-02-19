package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestrator_History_MoveToHistory(t *testing.T) {
	poller := newMockPoller([]WorkItem{{ID: "JOB-1", Summary: "Job 1"}})
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Run orchestrator
	err := orch.Run(ctx, silentLogger)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	// Verify job is in history
	orch.mu.RLock()
	assert.Empty(t, orch.activeJobs, "Job should not be active")
	assert.Len(t, orch.completedJobs, 1, "Job should be in history")
	job := orch.completedJobs[0]
	orch.mu.RUnlock()

	assert.Equal(t, "JOB-1", job.ID)
	assert.Equal(t, "Completed", job.Status)
	assert.False(t, job.EndTime.IsZero(), "EndTime should be set")
}

func TestOrchestrator_History_Limit(t *testing.T) {
	poller := newMockPoller(nil) // No initial items
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond)
	orch.maxHistory = 5 // Set small limit for testing

	// Manually add to history
	for i := 0; i < 7; i++ {
		id := string(rune('A' + i)) // A, B, C...
		job := JobInfo{ID: id, EndTime: time.Now(), Status: "Completed"}
		orch.addToHistory(job, nil)
	}

	orch.mu.RLock()
	assert.Len(t, orch.completedJobs, 5, "History should be capped at 5")
	// Expected content: C, D, E, F, G (A and B removed)
	assert.Equal(t, "C", orch.completedJobs[0].ID)
	assert.Equal(t, "G", orch.completedJobs[4].ID)
	orch.mu.RUnlock()
}

func TestOrchestrator_GetJob_History(t *testing.T) {
	orch := New(&mockPoller{}, &mockSpawner{}, 0)

	activeJob := JobInfo{ID: "ACTIVE", Status: "Running"}
	completedJob := JobInfo{ID: "COMPLETED", Status: "Completed"}

	orch.mu.Lock()
	orch.activeJobs["ACTIVE"] = activeJob
	orch.completedJobs = append(orch.completedJobs, completedJob)
	orch.mu.Unlock()

	// Test GetJob
	job, err := orch.GetJob("ACTIVE")
	assert.NoError(t, err)
	assert.Equal(t, "Running", job.Status)

	job, err = orch.GetJob("COMPLETED")
	assert.NoError(t, err)
	assert.Equal(t, "Completed", job.Status)

	_, err = orch.GetJob("MISSING")
	assert.Error(t, err)
}

func TestOrchestrator_History_FailedJob(t *testing.T) {
	poller := newMockPoller([]WorkItem{{ID: "FAIL-1"}})
	spawner := &mockSpawner{spawnErr: errors.New("spawn error")}
	orch := New(poller, spawner, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_ = orch.Run(ctx, silentLogger) // Expected to timeout

	orch.mu.RLock()
	// orch.Run waits for workers to finish on context cancellation,
	// so history should be populated by now.
	if len(orch.completedJobs) == 0 {
		// Wait a bit just in case
		time.Sleep(100 * time.Millisecond)
	}

	if assert.Len(t, orch.completedJobs, 1) {
		job := orch.completedJobs[0]
		assert.Equal(t, "FAIL-1", job.ID)
		assert.Equal(t, "Failed", job.Status)
		assert.Equal(t, "spawn error", job.Error)
	}
	orch.mu.RUnlock()
}
