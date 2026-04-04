package orchestrator

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCancelJobsOlderThan(t *testing.T) {
	poller := &MockPoller{}
	spawner := &MockSpawner{}
	orch := New(poller, spawner, time.Minute)
	logger := slog.Default()

	now := time.Now()
	orch.activeJobs = map[string]JobInfo{
		"active-1": {ID: "active-1", StartTime: now.Add(-2 * time.Hour)},
		"active-2": {ID: "active-2", StartTime: now.Add(-10 * time.Minute)},
	}
	orch.pendingJobs = map[string]JobInfo{
		"pending-1": {ID: "pending-1", StartTime: now.Add(-2 * time.Hour)},
		"pending-2": {ID: "pending-2", StartTime: now.Add(-10 * time.Minute)},
	}

	spawner.On("Cancel", context.Background(), "active-1").Return(nil)
	spawner.On("Cancel", context.Background(), "pending-1").Return(nil)

	count, err := orch.CancelJobsOlderThan(context.Background(), time.Hour, logger)

	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify pending-1 was removed
	orch.mu.RLock()
	_, existsPending1 := orch.pendingJobs["pending-1"]
	_, existsPending2 := orch.pendingJobs["pending-2"]
	orch.mu.RUnlock()

	assert.False(t, existsPending1)
	assert.True(t, existsPending2)
}
