package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type priorityMockSpawner struct {
	mu           sync.Mutex
	spawnedOrder []string
	blockChs     map[string]chan struct{}
}

func (m *priorityMockSpawner) Spawn(ctx context.Context, item WorkItem) error {
	m.mu.Lock()
	m.spawnedOrder = append(m.spawnedOrder, item.ID)
	ch, ok := m.blockChs[item.ID]
	m.mu.Unlock()

	if ok && ch != nil {
		<-ch
	}
	return nil
}

func (m *priorityMockSpawner) Cleanup(ctx context.Context, item WorkItem) error { return nil }
func (m *priorityMockSpawner) Cancel(ctx context.Context, jobID string) error   { return nil }
func (m *priorityMockSpawner) GetLogs(ctx context.Context, jobID string) (io.ReadCloser, error) {
	return nil, nil
}
func (m *priorityMockSpawner) Ping(ctx context.Context) error { return nil }

func TestOrchestrator_PrioritySorting(t *testing.T) {
	// 1. Initial job to block capacity
	itemBlock := WorkItem{ID: "BLOCKER", Priority: 0}

	// 2. Queue of jobs that will be polled while blocked
	itemLow := WorkItem{ID: "LOW", Priority: 1}
	itemHigh := WorkItem{ID: "HIGH", Priority: 10}
	itemMed := WorkItem{ID: "MED", Priority: 5}
	itemSame1 := WorkItem{ID: "SAME-A", Priority: 5} // Test stable sort by ID
	itemSame2 := WorkItem{ID: "SAME-B", Priority: 5}

	blockCh := make(chan struct{})
	spawner := &priorityMockSpawner{
		blockChs: map[string]chan struct{}{
			"BLOCKER": blockCh,
		},
	}

	poller := newMockPoller([]WorkItem{itemBlock})

	orch := New(poller, spawner, 20*time.Millisecond)
	orch.MaxConcurrentJobs = 1

	ctx, cancel := context.WithCancel(context.Background())
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Start orchestrator
	go orch.Run(ctx, logger)

	// Wait for blocker to spawn
	time.Sleep(50 * time.Millisecond)

	spawner.mu.Lock()
	assert.Equal(t, []string{"BLOCKER"}, spawner.spawnedOrder)
	spawner.mu.Unlock()

	// Now update poller to return the rest of the jobs
	// The orchestrator will try to process them, but hit capacity, so they should go to pending queue
	// Note: We don't actually need the poller to return them if we submit them manually!
	// Manual submission puts them directly into activeJobs or pendingJobs depending on DependsOn.
	// So we can just skip updating the poller entirely, since manual submission overrides it.

	// We can manually add them to pending queue via SubmitJob with a dependency that we clear, or just
	// let the poller fail ErrAtCapacity. Wait, if poller fails ErrAtCapacity, it drops them?
	// Let's check poller behavior in Run().
	// "If err == ErrAtCapacity { break }" - so it stops processing that batch, but next poll it gets them again.
	// But they are NOT put into pendingJobs if they hit ErrAtCapacity during normal `processWorkItem` unless they have dependencies!
	// Ah! PendingJobs is ONLY for dependencies! If ErrAtCapacity, they are rejected.
	// Let's verify `evaluatePendingJobs`. If a pending job is ready but hits capacity, it is put BACK in pendingJobs.
	// To test Priority queueing, we need them in pendingJobs.
	// The easiest way is to give them a dependency on "BLOCKER".

	itemLow.DependsOn = []string{"BLOCKER"}
	itemHigh.DependsOn = []string{"BLOCKER"}
	itemMed.DependsOn = []string{"BLOCKER"}
	itemSame1.DependsOn = []string{"BLOCKER"}
	itemSame2.DependsOn = []string{"BLOCKER"}

	// Manually submit them so they go to pending
	orch.SubmitJob(ctx, itemLow, logger)
	orch.SubmitJob(ctx, itemSame1, logger)
	orch.SubmitJob(ctx, itemMed, logger)
	orch.SubmitJob(ctx, itemHigh, logger)
	orch.SubmitJob(ctx, itemSame2, logger)

	orch.mu.RLock()
	assert.Len(t, orch.pendingJobs, 5)
	orch.mu.RUnlock()

	// Unblock "BLOCKER"
	close(blockCh)

	// Wait for jobs to be evaluated and spawned
	assert.Eventually(t, func() bool {
		spawner.mu.Lock()
		defer spawner.mu.Unlock()
		return len(spawner.spawnedOrder) == 6
	}, 2*time.Second, 10*time.Millisecond, "Expected 6 jobs to be spawned")

	spawner.mu.Lock()
	defer spawner.mu.Unlock()

	// Expected order: BLOCKER (0), HIGH (10), MED (5), SAME-A (5), SAME-B (5), LOW (1)
	// Because MaxConcurrentJobs=1, they should spawn one by one in priority order!
	// Wait, actually if MaxConcurrentJobs=1, evaluatePendingJobs will spawn HIGH, then hit capacity for MED, putting it BACK in pending!
	// Then HIGH finishes (it doesn't block), evaluate spawns MED, etc.
	// This perfectly tests the Priority sorting!

	assert.Equal(t, []string{"BLOCKER", "HIGH", "MED", "SAME-A", "SAME-B", "LOW"}, spawner.spawnedOrder)

	cancel()
}
