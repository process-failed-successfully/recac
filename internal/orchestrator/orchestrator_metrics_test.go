package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddJobMetrics(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{
		blockCh: make(chan struct{}),
	}
	orch := New(poller, spawner, 10*time.Millisecond)

	item := WorkItem{
		ID:      "METRICS-JOB",
		Summary: "Test metrics",
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()
	err := orch.SubmitJob(ctx, item, logger)
	require.NoError(t, err)

	// Test adding to active job
	err = orch.AddJobMetrics("METRICS-JOB", map[string]float64{"cost": 1.5, "tokens": 100}, logger)
	require.NoError(t, err)

	job, err := orch.GetJob("METRICS-JOB")
	require.NoError(t, err)
	assert.Equal(t, 1.5, job.Metrics["cost"])
	assert.Equal(t, float64(100), job.Metrics["tokens"])

	// Test adding to pending job
	orch.pendingJobs["PENDING-METRICS-JOB"] = JobInfo{
		ID:      "PENDING-METRICS-JOB",
		Summary: "Test pending metrics",
	}
	err = orch.AddJobMetrics("PENDING-METRICS-JOB", map[string]float64{"cost": 2.0}, logger)
	require.NoError(t, err)
	job, err = orch.GetJob("PENDING-METRICS-JOB")
	require.NoError(t, err)
	assert.Equal(t, 2.0, job.Metrics["cost"])

	// Test appending
	err = orch.AddJobMetrics("METRICS-JOB", map[string]float64{"cost": 0.5, "time": 10}, logger)
	require.NoError(t, err)

	job, err = orch.GetJob("METRICS-JOB")
	require.NoError(t, err)
	assert.Equal(t, 2.0, job.Metrics["cost"])
	assert.Equal(t, float64(100), job.Metrics["tokens"])
	assert.Equal(t, float64(10), job.Metrics["time"])

	// Let it finish
	close(spawner.blockCh)
	time.Sleep(50 * time.Millisecond) // Let goroutine finish

	// Test adding to completed job
	err = orch.AddJobMetrics("METRICS-JOB", map[string]float64{"cost": 1.0}, logger)
	require.NoError(t, err)

	job, err = orch.GetJob("METRICS-JOB")
	require.NoError(t, err)
	assert.Equal(t, 3.0, job.Metrics["cost"])

	// Test adding to non-existent job
	err = orch.AddJobMetrics("NON-EXISTENT", map[string]float64{"cost": 1.0}, logger)
	require.Error(t, err)

	// Test appending to completed job
	err = orch.AddJobMetrics("METRICS-JOB", map[string]float64{"cost": 1.0}, logger)
	require.NoError(t, err)

	job, err = orch.GetJob("METRICS-JOB")
	require.NoError(t, err)
	assert.Equal(t, 4.0, job.Metrics["cost"])
}

func TestGetAnalyticsMetrics(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 10*time.Millisecond)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	// Submit Job 1
	err := orch.SubmitJob(ctx, WorkItem{ID: "JOB-1", Summary: "J1"}, logger)
	require.NoError(t, err)

	// Ensure job 1 finishes quickly since spawner doesn't block
	time.Sleep(50 * time.Millisecond)

	err = orch.AddJobMetrics("JOB-1", map[string]float64{"cost": 10.0, "time": 5.0}, logger)
	require.NoError(t, err)

	// Submit Job 2
	err = orch.SubmitJob(ctx, WorkItem{ID: "JOB-2", Summary: "J2"}, logger)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	err = orch.AddJobMetrics("JOB-2", map[string]float64{"cost": 5.0, "other": 1.0}, logger)
	require.NoError(t, err)

	analytics := orch.GetAnalytics()

	assert.Equal(t, 2, analytics.TotalJobs)
	assert.Equal(t, 15.0, analytics.TotalMetrics["cost"])
	assert.Equal(t, 5.0, analytics.TotalMetrics["time"])
	assert.Equal(t, 1.0, analytics.TotalMetrics["other"])
}
