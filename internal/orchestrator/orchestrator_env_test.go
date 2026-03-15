package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"recac/internal/telemetry"
)

func TestUpdateJobEnv(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := new(MockSpawner)
	orch := New(poller, spawner, 1*time.Minute)
	orch.RequireApproval = true // to keep it pending approval
	ctx := context.Background()

	// 1. Setup Jobs
	activeItem := WorkItem{ID: "JOB-ACTIVE", Summary: "Active Job"}
	pendingItem := WorkItem{ID: "JOB-PENDING", Summary: "Pending Job"}
	historyItem := WorkItem{ID: "JOB-HISTORY", Summary: "History Job"}

	orch.mu.Lock()
	orch.activeJobs["JOB-ACTIVE"] = JobInfo{ID: "JOB-ACTIVE", Status: "Running", WorkItem: activeItem}
	orch.pendingJobs["JOB-PENDING"] = JobInfo{ID: "JOB-PENDING", Status: "Pending Approval", WorkItem: pendingItem}
	orch.completedJobs = append(orch.completedJobs, JobInfo{ID: "JOB-HISTORY", Status: "Completed", WorkItem: historyItem})
	orch.mu.Unlock()

	envVars := map[string]string{"ENV_KEY": "ENV_VAL"}

	// 2. Update existing pending job
	err := orch.UpdateJobEnv(ctx, "JOB-PENDING", envVars, nil)
	require.NoError(t, err)

	orch.mu.Lock()
	job := orch.pendingJobs["JOB-PENDING"]
	orch.mu.Unlock()
	assert.Equal(t, envVars, job.WorkItem.EnvVars)

	// 3. Update active job
	err = orch.UpdateJobEnv(ctx, "JOB-ACTIVE", envVars, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already active")

	// 4. Update completed job
	err = orch.UpdateJobEnv(ctx, "JOB-HISTORY", envVars, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already completed")

	// 5. Update nonexistent job
	err = orch.UpdateJobEnv(ctx, "NON-EXISTENT", envVars, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUpdateJobsEnvBulk(t *testing.T) {
	ctx := context.Background()
	logger := telemetry.NewLogger(false, "test", false)
	poller := &MockPoller{}
	spawner := &MockSpawner{}
	orch := New(poller, spawner, time.Second)

	// Create pending jobs
	job1 := WorkItem{
		ID:       "JOB-1",
		Summary:  "Update Job 1",
		Tags:     []string{"backend", "urgent"},
		EnvVars:  map[string]string{"OLD": "VAL1"},
		RunAfter: time.Now().Add(10 * time.Minute), // keep it pending
	}
	job2 := WorkItem{
		ID:       "JOB-2",
		Summary:  "Fix database bug",
		Tags:     []string{"database"},
		EnvVars:  map[string]string{"OLD": "VAL2"},
		RunAfter: time.Now().Add(10 * time.Minute), // keep it pending
	}
	job3 := WorkItem{
		ID:       "JOB-3",
		Summary:  "Another backend job",
		Tags:     []string{"backend"},
		EnvVars:  map[string]string{"OLD": "VAL3"},
		RunAfter: time.Now().Add(10 * time.Minute), // keep it pending
	}

	require.NoError(t, orch.SubmitJob(ctx, job1, logger))
	require.NoError(t, orch.SubmitJob(ctx, job2, logger))
	require.NoError(t, orch.SubmitJob(ctx, job3, logger))

	// Test UpdateJobsEnvByTag
	count, err := orch.UpdateJobsEnvByTag(ctx, "backend", map[string]string{"NEW_TAG": "YES"}, logger)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	pendingJobs := orch.GetPendingJobs()
	for _, job := range pendingJobs {
		if job.ID == "JOB-1" || job.ID == "JOB-3" {
			require.Equal(t, "YES", job.WorkItem.EnvVars["NEW_TAG"])
		} else if job.ID == "JOB-2" {
			_, exists := job.WorkItem.EnvVars["NEW_TAG"]
			require.False(t, exists)
		}
	}

	// Test UpdateJobsEnvByMatch
	countMatch, errMatch := orch.UpdateJobsEnvByMatch(ctx, "Fix", map[string]string{"MATCHED": "TRUE"}, logger)
	require.NoError(t, errMatch)
	require.Equal(t, 1, countMatch)

	pendingJobs2 := orch.GetPendingJobs()
	for _, job := range pendingJobs2 {
		if job.ID == "JOB-2" {
			require.Equal(t, "TRUE", job.WorkItem.EnvVars["MATCHED"])
		} else {
			_, exists := job.WorkItem.EnvVars["MATCHED"]
			require.False(t, exists)
		}
	}

	// Test No match
	countNoMatch, errNoMatch := orch.UpdateJobsEnvByMatch(ctx, "Nonexistent", map[string]string{"XXX": "YYY"}, logger)
	require.NoError(t, errNoMatch)
	require.Equal(t, 0, countNoMatch)
}
