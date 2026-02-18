package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestrator_DryRun_Success(t *testing.T) {
	expectedItems := []WorkItem{
		{ID: "DRY-1", Summary: "Dry Run Task 1"},
		{ID: "DRY-2", Summary: "Dry Run Task 2"},
	}
	poller := newMockPoller(expectedItems)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 10*time.Millisecond)

	ctx := context.Background()
	items, err := orch.DryRun(ctx, silentLogger)

	require.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, expectedItems[0].ID, items[0].ID)
	assert.Equal(t, expectedItems[1].ID, items[1].ID)

	// Verify nothing was spawned
	spawner.mu.Lock()
	defer spawner.mu.Unlock()
	assert.Empty(t, spawner.spawned, "DryRun should not spawn agents")
}

func TestOrchestrator_DryRun_PollError(t *testing.T) {
	poller := newMockPoller(nil)
	poller.pollErr = errors.New("poll error simulation")
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 10*time.Millisecond)

	ctx := context.Background()
	items, err := orch.DryRun(ctx, silentLogger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "dry run poll failed")
	assert.Contains(t, err.Error(), "poll error simulation")
	assert.Nil(t, items)

	// Verify nothing was spawned
	spawner.mu.Lock()
	defer spawner.mu.Unlock()
	assert.Empty(t, spawner.spawned, "DryRun should not spawn agents")
}

func TestOrchestrator_DryRun_NoWork(t *testing.T) {
	poller := newMockPoller(nil) // No items
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 10*time.Millisecond)

	ctx := context.Background()
	items, err := orch.DryRun(ctx, silentLogger)

	require.NoError(t, err)
	assert.Empty(t, items)

	// Verify nothing was spawned
	spawner.mu.Lock()
	defer spawner.mu.Unlock()
	assert.Empty(t, spawner.spawned, "DryRun should not spawn agents")
}
