package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"recac/internal/telemetry"
)

import (
	"github.com/stretchr/testify/mock"
)

func TestOrchestrator_JobDelayAfterDependency(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	mockSpawner.On("GetLogs", mock.Anything, mock.Anything).Return(nil, nil).Maybe()

	logger := telemetry.NewLogger(true, "test-orch-delay", false)

	o := New(mockPoller, mockSpawner, 100*time.Millisecond)

	// We'll set up two jobs.
	// Job 1 has no dependencies.
	// Job 2 depends on Job 1 and has a Delay of 500ms.

	job1 := WorkItem{
		ID:      "JOB-1",
		Summary: "First Job",
	}

	job2 := WorkItem{
		ID:        "JOB-2",
		Summary:   "Second Job",
		DependsOn: []string{"JOB-1"},
		Delay:     500 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Submit both jobs
	err := o.SubmitJob(ctx, job1, logger)
	require.NoError(t, err)

	err = o.SubmitJob(ctx, job2, logger)
	require.NoError(t, err)

	// Wait a moment for goroutines
	// Poll to avoid race conditions with time.Sleep
	var pending []JobInfo
	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		pending = o.GetPendingJobs()
		if len(pending) == 1 && pending[0].ID == "JOB-2" {
			break
		}
	}

	// Because mockSpawner finishes immediately and clears active jobs,
	// we just need to ensure JOB-1 completed and JOB-2 is still pending due to delay.

	require.Len(t, pending, 1)
	assert.Equal(t, "JOB-2", pending[0].ID)

	o.mu.RLock()
	j2Info := o.pendingJobs["JOB-2"]
	runAfter := j2Info.WorkItem.RunAfter
	delayRemaining := j2Info.WorkItem.Delay
	o.mu.RUnlock()

	assert.False(t, runAfter.IsZero(), "RunAfter should be set once dependencies are met")
	assert.Equal(t, time.Duration(0), delayRemaining, "Delay should be reset to 0 to prevent re-applying")

	// Wait 100ms, it should STILL be pending
	time.Sleep(100 * time.Millisecond)
	pending = o.GetPendingJobs()
	assert.Len(t, pending, 1)

	// Wait until delay expires
	// Use polling again for robustness
	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		pending = o.GetPendingJobs()
		if len(pending) == 0 {
			break
		}
	}

	// It should now be spawned and completed
	assert.Len(t, pending, 0)

	completed := o.GetCompletedJobs()
	foundJ2 := false
	for _, c := range completed {
		if c.ID == "JOB-2" {
			foundJ2 = true
			break
		}
	}
	assert.True(t, foundJ2, "JOB-2 should have completed after delay")
}
