package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
