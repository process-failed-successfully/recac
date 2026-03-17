package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUpdateJobTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		orch := New(nil, nil, time.Minute)

		// Add a job to pendingJobs, set RunAfter to ensure it stays pending
		orch.pendingJobs["job1"] = JobInfo{
			ID: "job1",
			WorkItem: WorkItem{
				ID:       "job1",
				Timeout:  10 * time.Minute,
				RunAfter: time.Now().Add(1 * time.Hour),
			},
		}

		err := orch.UpdateJobTimeout(ctx, "job1", 20*time.Minute, logger)
		assert.NoError(t, err)

		// Verify timeout was updated
		orch.mu.RLock()
		job := orch.pendingJobs["job1"]
		orch.mu.RUnlock()
		assert.Equal(t, 20*time.Minute, job.WorkItem.Timeout)
	})

	t.Run("JobNotFound", func(t *testing.T) {
		orch := New(nil, nil, time.Minute)

		err := orch.UpdateJobTimeout(ctx, "missing-job", 20*time.Minute, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found in pending queue")
	})

	t.Run("JobAlreadyActive", func(t *testing.T) {
		orch := New(nil, nil, time.Minute)

		// Add a job to activeJobs
		orch.activeJobs["job1"] = JobInfo{
			ID: "job1",
			WorkItem: WorkItem{
				ID: "job1",
			},
		}

		err := orch.UpdateJobTimeout(ctx, "job1", 20*time.Minute, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already active and cannot have timeout updated")
	})

	t.Run("JobAlreadyCompleted", func(t *testing.T) {
		orch := New(nil, nil, time.Minute)

		// Add a job to completedJobs
		orch.completedJobs = append(orch.completedJobs, JobInfo{
			ID: "job1",
			WorkItem: WorkItem{
				ID: "job1",
			},
		})

		err := orch.UpdateJobTimeout(ctx, "job1", 20*time.Minute, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already completed")
	})
}

func TestUpdateJobsTimeoutBulk(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	setupOrch := func() *Orchestrator {
		orch := New(nil, nil, time.Minute)
		orch.pendingJobs = map[string]JobInfo{
			"job1": {
				ID: "job1",
				Summary: "Fix login bug",
				WorkItem: WorkItem{
					ID:       "job1",
					Tags:     []string{"backend", "urgent"},
					Timeout:  10 * time.Minute,
					RunAfter: time.Now().Add(1 * time.Hour),
				},
			},
			"job2": {
				ID: "job2",
				Summary: "Update UI",
				WorkItem: WorkItem{
					ID:       "job2",
					Tags:     []string{"frontend"},
					Timeout:  10 * time.Minute,
					RunAfter: time.Now().Add(1 * time.Hour),
				},
			},
			"job3": {
				ID: "job3",
				Summary: "Fix database issue",
				WorkItem: WorkItem{
					ID:       "job3",
					Tags:     []string{"backend"},
					Timeout:  10 * time.Minute,
					RunAfter: time.Now().Add(1 * time.Hour),
				},
			},
		}
		return orch
	}

	t.Run("UpdateJobsTimeoutByTag", func(t *testing.T) {
		orch := setupOrch()

		count, err := orch.UpdateJobsTimeoutByTag(ctx, "backend", 30*time.Minute, logger)
		assert.NoError(t, err)
		assert.Equal(t, 2, count)

		orch.mu.RLock()
		defer orch.mu.RUnlock()

		assert.Equal(t, 30*time.Minute, orch.pendingJobs["job1"].WorkItem.Timeout)
		assert.Equal(t, 10*time.Minute, orch.pendingJobs["job2"].WorkItem.Timeout)
		assert.Equal(t, 30*time.Minute, orch.pendingJobs["job3"].WorkItem.Timeout)
	})

	t.Run("UpdateJobsTimeoutByMatch", func(t *testing.T) {
		orch := setupOrch()

		countMatch, errMatch := orch.UpdateJobsTimeoutByMatch(ctx, "Fix", 45*time.Minute, logger)
		assert.NoError(t, errMatch)
		assert.Equal(t, 2, countMatch)

		orch.mu.RLock()
		defer orch.mu.RUnlock()

		assert.Equal(t, 45*time.Minute, orch.pendingJobs["job1"].WorkItem.Timeout)
		assert.Equal(t, 10*time.Minute, orch.pendingJobs["job2"].WorkItem.Timeout)
		assert.Equal(t, 45*time.Minute, orch.pendingJobs["job3"].WorkItem.Timeout)
	})

	t.Run("UpdateJobsTimeoutByMatch_InvalidRegex", func(t *testing.T) {
		orch := setupOrch()

		count, err := orch.UpdateJobsTimeoutByMatch(ctx, "[invalid", 45*time.Minute, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid match regex")
		assert.Equal(t, 0, count)
	})

	t.Run("UpdateJobsTimeoutByMatch_NoMatch", func(t *testing.T) {
		orch := setupOrch()

		countNoMatch, errNoMatch := orch.UpdateJobsTimeoutByMatch(ctx, "Nonexistent", 60*time.Minute, logger)
		assert.NoError(t, errNoMatch)
		assert.Equal(t, 0, countNoMatch)

		orch.mu.RLock()
		defer orch.mu.RUnlock()

		assert.Equal(t, 10*time.Minute, orch.pendingJobs["job1"].WorkItem.Timeout)
		assert.Equal(t, 10*time.Minute, orch.pendingJobs["job2"].WorkItem.Timeout)
		assert.Equal(t, 10*time.Minute, orch.pendingJobs["job3"].WorkItem.Timeout)
	})
}
