package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestrator_CircuitBreakerTrips(t *testing.T) {
	poller := newMockPoller([]WorkItem{
		{ID: "JOB-1"},
		{ID: "JOB-2"},
		{ID: "JOB-3"},
	})
	spawner := &mockSpawner{spawnErr: errors.New("spawn failed")}
	orch := New(poller, spawner, 10*time.Millisecond)
	orch.CircuitBreakerMaxFailures = 2 // Should trip after 2 failures

	// Setup a subscriber to verify the event is broadcasted
	ch := orch.Subscribe()
	defer orch.Unsubscribe(ch)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := orch.Run(ctx, silentLogger)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	status := orch.GetStatus()
	assert.True(t, status.CircuitBroken, "Circuit should be broken after 2 consecutive spawn failures")
	assert.True(t, status.Paused, "Orchestrator should be paused when circuit is broken")
	assert.GreaterOrEqual(t, orch.ConsecutiveSpawnFailures, 2, "Should have at least 2 consecutive spawn failures")

	// Verify the event was broadcasted
	eventFound := false
	for {
		select {
		case evt := <-ch:
			if string(evt) != "" {
				if strings.Contains(string(evt), "circuit_breaker_tripped") {
					eventFound = true
				}
			}
		default:
			goto CheckEvent
		}
	}
CheckEvent:
	assert.True(t, eventFound, "circuit_breaker_tripped event should have been broadcasted")
}

func TestOrchestrator_CircuitBreakerResetsOnSuccess(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 10*time.Millisecond)
	orch.CircuitBreakerMaxFailures = 3

	orch.recordSpawnFailure(silentLogger)
	orch.recordSpawnFailure(silentLogger)
	assert.Equal(t, 2, orch.ConsecutiveSpawnFailures)
	assert.False(t, orch.CircuitBroken)

	orch.recordSpawnSuccess()
	assert.Equal(t, 0, orch.ConsecutiveSpawnFailures, "Consecutive failures should reset on success")
	assert.False(t, orch.CircuitBroken)
}

func TestOrchestrator_CircuitBreakerResume(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 10*time.Millisecond)

	orch.CircuitBroken = true
	orch.paused = true
	orch.ConsecutiveSpawnFailures = 5

	orch.Resume()

	assert.False(t, orch.CircuitBroken, "Resume should reset CircuitBroken")
	assert.False(t, orch.paused, "Resume should reset paused")
	assert.Equal(t, 0, orch.ConsecutiveSpawnFailures, "Resume should reset consecutive failures")
}
