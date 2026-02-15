package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockObserver struct {
	pollStarted  bool
	pollEnded    bool
	spawnStarted bool
	spawnEnded   bool
	spawnedItem  WorkItem
}

func (m *mockObserver) OnPollStart() {
	m.pollStarted = true
}

func (m *mockObserver) OnPollEnd(items []WorkItem, err error) {
	m.pollEnded = true
}

func (m *mockObserver) OnSpawnStart(item WorkItem) {
	m.spawnStarted = true
	m.spawnedItem = item
}

func (m *mockObserver) OnSpawnEnd(item WorkItem, err error) {
	m.spawnEnded = true
}

func TestOrchestrator_ObserverCalls(t *testing.T) {
	item := WorkItem{ID: "TEST-OBS"}
	poller := newMockPoller([]WorkItem{item})
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 10*time.Millisecond)

	obs := &mockObserver{}
	orch.SetObserver(obs)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := orch.Run(ctx, silentLogger)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	// Verify Observer calls
	assert.True(t, obs.pollStarted, "OnPollStart should be called")
	assert.True(t, obs.pollEnded, "OnPollEnd should be called")
	// Since Run spawns asynchronously, we might need to wait for spawn logic to complete.
	// But in mockSpawner/Orchestrator loop, the observer calls are synchronous around Spawner.Spawn.
	// However, Orchestrator spawns inside a goroutine.
	// The test waits for timeout (50ms). The goroutine should have run.

	// Wait a bit to ensure goroutines finish (though 50ms should be enough)
	time.Sleep(10*time.Millisecond)

	assert.True(t, obs.spawnStarted, "OnSpawnStart should be called")
	assert.True(t, obs.spawnEnded, "OnSpawnEnd should be called")
	assert.Equal(t, item.ID, obs.spawnedItem.ID)
}
