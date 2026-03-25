package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"fmt"
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

type mockMetricsPersistence struct {
	savedJobs map[string]JobInfo
	saveErr   error
	getErr    error
}

func (m *mockMetricsPersistence) Init() error { return nil }
func (m *mockMetricsPersistence) SaveJob(job JobInfo) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	if m.savedJobs == nil {
		m.savedJobs = make(map[string]JobInfo)
	}
	m.savedJobs[job.ID] = job
	return nil
}
func (m *mockMetricsPersistence) GetJob(id string) (*JobInfo, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.savedJobs != nil {
		if job, ok := m.savedJobs[id]; ok {
			return &job, nil
		}
	}
	return nil, fmt.Errorf("job %s not found", id)
}
func (m *mockMetricsPersistence) GetJobs(limit int) ([]JobInfo, error) { return nil, nil }
func (m *mockMetricsPersistence) Close() error { return nil }
func (m *mockMetricsPersistence) ClearHistory() (int, error) { return 0, nil }

func TestAddJobMetrics_Persistence(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 10*time.Millisecond)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	jobID := "PERSISTED-JOB"

	t.Run("Success", func(t *testing.T) {
		mockDB := &mockMetricsPersistence{
			savedJobs: map[string]JobInfo{
				jobID: {ID: jobID},
			},
		}
		orch.SetPersistence(mockDB)

		err := orch.AddJobMetrics(jobID, map[string]float64{"cost": 1.0}, logger)
		require.NoError(t, err)

		job, err := mockDB.GetJob(jobID)
		require.NoError(t, err)
		assert.Equal(t, 1.0, job.Metrics["cost"])
	})

	t.Run("SaveJobError", func(t *testing.T) {
		mockDB := &mockMetricsPersistence{
			savedJobs: map[string]JobInfo{
				jobID: {ID: jobID},
			},
			saveErr: assert.AnError,
		}
		orch.SetPersistence(mockDB)

		err := orch.AddJobMetrics(jobID, map[string]float64{"cost": 1.0}, logger)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to persist")
	})

	t.Run("MemoryAndSaveJobError", func(t *testing.T) {
		mockDB := &mockMetricsPersistence{
			saveErr: assert.AnError,
		}
		orch.SetPersistence(mockDB)

		// Test when the job is in memory history but persistence fails
		orch.completedJobs = append(orch.completedJobs, JobInfo{ID: "MEM-JOB"})

		err := orch.AddJobMetrics("MEM-JOB", map[string]float64{"cost": 1.0}, logger)
		require.NoError(t, err) // It continues despite persistence failure

		// Need to remove MEM-JOB from memory for next tests if we reuse orch, but we are re-instantiating.
		// Wait, we reuse orch!
		// Let's remove it.
		orch.completedJobs = orch.completedJobs[:len(orch.completedJobs)-1]
	})
}
func (m *mockMetricsPersistence) PurgeJob(id string) error { return nil }
