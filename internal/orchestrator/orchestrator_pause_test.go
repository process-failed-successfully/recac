package orchestrator

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOrchestrator_PauseResume(t *testing.T) {
	// Setup poller with enough items for multiple polls
	poller := newMockPoller([]WorkItem{
		{ID: "TEST-1", Summary: "Task 1"},
		{ID: "TEST-2", Summary: "Task 2"},
		{ID: "TEST-3", Summary: "Task 3"},
	})
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond) // Fast poll interval

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := orch.Run(ctx, silentLogger)
		if err != nil && err != context.Canceled {
			t.Errorf("Orchestrator failed: %v", err)
		}
	}()

	// 1. Let it run for a bit to pick up initial items
	time.Sleep(100 * time.Millisecond)

	// Pause!
	orch.Pause()

	// Check status
	status := orch.GetStatus()
	assert.True(t, status.Paused, "Orchestrator should be paused")

	// Capture spawned count at pause time
	spawner.mu.Lock()
	countAtPause := len(spawner.spawned)
	spawner.mu.Unlock()

	// 2. Wait while paused - should NOT pick up more work
	time.Sleep(200 * time.Millisecond)

	spawner.mu.Lock()
	countAfterWait := len(spawner.spawned)
	spawner.mu.Unlock()

	assert.Equal(t, countAtPause, countAfterWait, "Should not spawn new items while paused")

	// Resume!
	orch.Resume()

	// Check status
	status = orch.GetStatus()
	assert.False(t, status.Paused, "Orchestrator should be resumed")

	// 3. Wait for resume to pick up more work
	// We need to make sure there IS more work.
	// The mock poller clears items after poll.
	// So we need to add more items to the mock poller.
	poller.itemsMu.Lock()
	poller.items["TEST-4"] = WorkItem{ID: "TEST-4", Summary: "Task 4"}
	poller.itemsMu.Unlock()

	time.Sleep(150 * time.Millisecond)

	spawner.mu.Lock()
	countFinal := len(spawner.spawned)
	spawner.mu.Unlock()

	assert.Greater(t, countFinal, countAfterWait, "Should spawn new items after resume")

	cancel()
	wg.Wait()
}
