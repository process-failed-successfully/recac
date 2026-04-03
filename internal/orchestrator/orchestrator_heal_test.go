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

func TestOrchestrator_HealJobs_Match(t *testing.T) {
	// Setup
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 10*time.Millisecond)

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Add a few completed/failed jobs to history
	job1 := JobInfo{
		ID:      "JOB-1",
		Status:  "Failed",
		Summary: "Test job that failed",
		Error:   "some weird error",
		WorkItem: WorkItem{
			ID:          "JOB-1",
			Summary:     "Test job that failed",
			Description: "Original description.",
		},
	}
	job2 := JobInfo{
		ID:      "JOB-2",
		Status:  "Failed",
		Summary: "Another job",
		Error:   "timeout error",
		WorkItem: WorkItem{
			ID:      "JOB-2",
			Summary: "Another job",
		},
	}
	job3 := JobInfo{
		ID:      "JOB-3",
		Status:  "Completed",
		Summary: "Test job that succeeded",
		WorkItem: WorkItem{
			ID:      "JOB-3",
			Summary: "Test job that succeeded",
		},
	}
	orch.addToHistory(job1, nil)
	orch.addToHistory(job2, nil)
	orch.addToHistory(job3, nil)

	// Act
	count, err := orch.HealJobs(ctx, "weird error", "", logger)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify the healed job is in pending Jobs with new ID and tag
	healedJob, err := orch.GetJob("JOB-1-healed")
	require.NoError(t, err)
	assert.Contains(t, healedJob.WorkItem.Description, "Previous Job Failure Context:")
	assert.Contains(t, healedJob.WorkItem.Description, "some weird error")
	assert.Contains(t, healedJob.WorkItem.Tags, "auto-heal")
}

func TestOrchestrator_HealJob(t *testing.T) {
	// Setup
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 10*time.Millisecond)

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Add a few completed/failed jobs to history
	job1 := JobInfo{
		ID:      "JOB-1",
		Status:  "Failed",
		Summary: "Test job that failed",
		Error:   "some weird error",
		WorkItem: WorkItem{
			ID:          "JOB-1",
			Summary:     "Test job that failed",
			Description: "Original description.",
		},
	}
	job2 := JobInfo{
		ID:      "JOB-2",
		Status:  "Completed",
		Summary: "Another job",
		WorkItem: WorkItem{
			ID:      "JOB-2",
			Summary: "Another job",
		},
	}
	orch.addToHistory(job1, nil)
	orch.addToHistory(job2, nil)

	// Act
	newID, err := orch.HealJob(ctx, "JOB-1", logger)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "JOB-1-healed", newID)

	// Verify the healed job is in pending Jobs with new ID and tag
	healedJob, err := orch.GetJob("JOB-1-healed")
	require.NoError(t, err)
	assert.Contains(t, healedJob.WorkItem.Description, "Original description.")
	assert.Contains(t, healedJob.WorkItem.Description, "Previous Job Failure Context:")
	assert.Contains(t, healedJob.WorkItem.Description, "some weird error")
	assert.Contains(t, healedJob.WorkItem.Tags, "auto-heal")

	// Test failure cases
	_, err = orch.HealJob(ctx, "JOB-2", logger)
	assert.ErrorContains(t, err, "not in a failed state")

	_, err = orch.HealJob(ctx, "JOB-UNKNOWN", logger)
	assert.ErrorContains(t, err, "not found")
}

func TestOrchestrator_HealJobs_Tag(t *testing.T) {
	// Setup
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 10*time.Millisecond)

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Add failed jobs
	job1 := JobInfo{
		ID:      "JOB-A",
		Status:  "Failed",
		WorkItem: WorkItem{
			ID:   "JOB-A",
			Tags: []string{"flaky"},
		},
	}
	job2 := JobInfo{
		ID:      "JOB-B",
		Status:  "Failed",
		WorkItem: WorkItem{
			ID:   "JOB-B",
			Tags: []string{"stable"},
		},
	}
	orch.addToHistory(job1, nil)
	orch.addToHistory(job2, nil)

	// Act
	count, err := orch.HealJobs(ctx, "", "flaky", logger)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	healedJob, err := orch.GetJob("JOB-A-healed")
	require.NoError(t, err)
	assert.Contains(t, healedJob.WorkItem.Tags, "auto-heal")
	assert.Contains(t, healedJob.WorkItem.Tags, "flaky")
}
