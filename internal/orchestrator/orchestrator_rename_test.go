package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOrchestrator_RenameJob(t *testing.T) {
	// Setup
	poller := new(MockPoller)
	spawner := new(MockSpawner)
	orch := New(poller, spawner, 1*time.Second)
	ctx := context.Background()

	// Add a pending job
	job := WorkItem{
		ID:        "old-id",
		Summary:   "Test Rename",
		DependsOn: []string{"never-met-dependency"}, // Ensure it stays pending
	}
	err := orch.SubmitJob(ctx, job, nil)
	require.NoError(t, err)

	// Check it exists
	j, err := orch.GetJob("old-id")
	require.NoError(t, err)
	require.Equal(t, "old-id", j.ID)

	// Add another job that depends on old-id
	dependentJob := WorkItem{
		ID:        "dependent-id",
		Summary:   "Dependent",
		DependsOn: []string{"old-id"},
	}
	err = orch.SubmitJob(ctx, dependentJob, nil)
	require.NoError(t, err)

	// Rename the first job
	err = orch.RenameJob(ctx, "old-id", "new-id", nil)
	require.NoError(t, err)

	// Verify old ID is gone
	_, err = orch.GetJob("old-id")
	require.ErrorContains(t, err, "not found")

	// Verify new ID exists and has correct data
	j2, err := orch.GetJob("new-id")
	require.NoError(t, err)
	require.Equal(t, "new-id", j2.ID)
	require.Equal(t, "new-id", j2.WorkItem.ID)
	require.Equal(t, "Test Rename", j2.WorkItem.Summary)

	// Verify dependent job was updated
	jDep, err := orch.GetJob("dependent-id")
	require.NoError(t, err)
	require.Equal(t, []string{"new-id"}, jDep.WorkItem.DependsOn)

	// Try renaming to an existing ID
	err = orch.SubmitJob(ctx, WorkItem{ID: "another-id", Summary: "Another", DependsOn: []string{"never-met-dependency"}}, nil)
	require.NoError(t, err)

	err = orch.RenameJob(ctx, "another-id", "new-id", nil)
	require.ErrorContains(t, err, "job new-id already exists in pending queue")

	// Try renaming a non-existent job
	err = orch.RenameJob(ctx, "missing-id", "brand-new-id", nil)
	require.ErrorContains(t, err, "job missing-id not found")
}
