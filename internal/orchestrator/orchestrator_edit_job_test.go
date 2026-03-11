package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOrchestrator_UpdateJobWorkItem(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 1*time.Minute)

	ctx := context.Background()
	item := WorkItem{
		ID:       "JOB-1",
		Summary:  "Old Summary",
		DependsOn: []string{"dep1"},
	}

	// Add to pending jobs directly
	orch.pendingJobs["JOB-1"] = JobInfo{
		ID:       "JOB-1",
		Summary:  item.Summary,
		Status:   "Pending",
		WorkItem: item,
	}

	newItem := WorkItem{
		ID:       "JOB-1",
		Summary:  "New Summary",
		DependsOn: []string{"dep2"},
		EnvVars: map[string]string{"foo": "bar"},
		Tags: []string{"tag1"},
	}

	err := orch.UpdateJobWorkItem(ctx, "JOB-1", newItem, nil)
	assert.NoError(t, err)

	updatedJob, err := orch.GetJob("JOB-1")
	assert.NoError(t, err)
	assert.Equal(t, "New Summary", updatedJob.Summary)
	assert.Equal(t, []string{"dep2"}, updatedJob.WorkItem.DependsOn)
	assert.Equal(t, map[string]string{"foo": "bar"}, updatedJob.WorkItem.EnvVars)
	assert.Equal(t, []string{"tag1"}, updatedJob.WorkItem.Tags)

	// Test changing ID
	newItemBadID := newItem
	newItemBadID.ID = "JOB-2"
	err = orch.UpdateJobWorkItem(ctx, "JOB-1", newItemBadID, nil)
	assert.ErrorContains(t, err, "cannot change job ID")

	// Test active job
	orch.activeJobs["JOB-2"] = JobInfo{ID: "JOB-2", Status: "Running"}
	err = orch.UpdateJobWorkItem(ctx, "JOB-2", WorkItem{ID: "JOB-2"}, nil)
	assert.ErrorContains(t, err, "already active and cannot be updated")

	// Test completed job
	orch.completedJobs = append(orch.completedJobs, JobInfo{ID: "JOB-3", Status: "Completed"})
	err = orch.UpdateJobWorkItem(ctx, "JOB-3", WorkItem{ID: "JOB-3"}, nil)
	assert.ErrorContains(t, err, "already completed")

	// Test unknown job
	err = orch.UpdateJobWorkItem(ctx, "JOB-4", WorkItem{ID: "JOB-4"}, nil)
	assert.ErrorContains(t, err, "not found in pending queue")

	// Test circular dependency revert
	orch.pendingJobs["JOB-5"] = JobInfo{ID: "JOB-5", WorkItem: WorkItem{ID: "JOB-5", DependsOn: []string{"JOB-1"}}}

	cycleItem := WorkItem{
		ID: "JOB-1",
		DependsOn: []string{"JOB-5"},
	}
	err = orch.UpdateJobWorkItem(ctx, "JOB-1", cycleItem, nil)
	assert.ErrorContains(t, err, "circular dependency detected")

	// Ensure it reverted
	updatedJob, _ = orch.GetJob("JOB-1")
	assert.Equal(t, "New Summary", updatedJob.Summary)
}
