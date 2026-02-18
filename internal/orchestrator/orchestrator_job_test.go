package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestOrchestrator_JobTracking tests that active jobs are correctly tracked
func TestOrchestrator_JobTracking(t *testing.T) {
	// 1. Setup
	item := WorkItem{ID: "JOB-123", Summary: "Fix bug"}
	poller := newMockPoller([]WorkItem{item})

	blockCh := make(chan struct{})
	spawner := &blockingSpawner{blockCh: blockCh}

	interval := 100 * time.Millisecond
	orch := New(poller, spawner, interval)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. Start Orchestrator
	go func() {
		orch.Run(ctx, silentLogger)
	}()

	// 3. Wait for poll and spawn (blocking)
	// Give it enough time to poll and start the goroutine
	time.Sleep(150 * time.Millisecond)

	// 4. Verify Job is Active
	jobs := orch.GetActiveJobs()
	assert.Len(t, jobs, 1, "Should have 1 active job")
	if len(jobs) > 0 {
		assert.Equal(t, "JOB-123", jobs[0].ID)
		assert.Equal(t, "Fix bug", jobs[0].Summary)
		assert.Equal(t, "Spawning", jobs[0].Status)
	}

	// 5. Unblock Spawner
	close(blockCh)

	// 6. Wait for completion
	time.Sleep(50 * time.Millisecond)

	// 7. Verify Job is Gone
	jobs = orch.GetActiveJobs()
	assert.Len(t, jobs, 0, "Should have 0 active jobs after completion")
}
