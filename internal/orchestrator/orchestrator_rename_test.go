package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestOrchestrator_RenameJob(t *testing.T) {
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)
	orch := New(&MockPoller{}, mockSpawner, 1*time.Minute)
	orch.RequireApproval = true // Prevent it from running immediately
	ctx := context.Background()

	// Submit some jobs
	job1 := WorkItem{ID: "JOB-1", Summary: "S1"}
	job2 := WorkItem{ID: "JOB-2", Summary: "S2", DependsOn: []string{"JOB-1"}}

	err := orch.SubmitJob(ctx, job1, nil)
	assert.NoError(t, err)
	err = orch.SubmitJob(ctx, job2, nil)
	assert.NoError(t, err)

	t.Run("successful rename", func(t *testing.T) {
		err := orch.RenameJob(ctx, "JOB-1", "NEW-JOB-1", nil)
		assert.NoError(t, err)

		pending := orch.GetPendingJobs()
		assert.Len(t, pending, 2)

		var found bool
		var depFound bool
		for _, j := range pending {
			if j.ID == "NEW-JOB-1" {
				found = true
				assert.Equal(t, "NEW-JOB-1", j.WorkItem.ID)
			}
			if j.ID == "JOB-2" {
				depFound = true
				assert.Contains(t, j.WorkItem.DependsOn, "NEW-JOB-1")
				assert.NotContains(t, j.WorkItem.DependsOn, "JOB-1")
			}
		}
		assert.True(t, found, "Renamed job not found")
		assert.True(t, depFound, "Dependent job not found")
	})

	t.Run("rename non-existent job", func(t *testing.T) {
		err := orch.RenameJob(ctx, "NON-EXISTENT", "SOME-NEW-ID", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found in pending queue")
	})

	t.Run("rename to existing job ID", func(t *testing.T) {
		err := orch.RenameJob(ctx, "JOB-2", "NEW-JOB-1", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("rename to same ID", func(t *testing.T) {
		err := orch.RenameJob(ctx, "JOB-2", "JOB-2", nil)
		assert.NoError(t, err)
	})
}
