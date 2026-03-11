package orchestrator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOrchestrator_PurgeJobsByStatus(t *testing.T) {
	orch := New(newMockPoller(nil), &mockSpawner{}, 50*time.Millisecond)

	job1 := JobInfo{ID: "JOB-1", Status: "Failed"}
	job2 := JobInfo{ID: "JOB-2", Status: "Completed"}

	orch.completedJobs = append(orch.completedJobs, job1, job2)

	count, err := orch.PurgeJobsByStatus("Failed", silentLogger)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	completed := orch.GetCompletedJobs()
	assert.Len(t, completed, 1)
	assert.Equal(t, "JOB-2", completed[0].ID)
}

func TestOrchestrator_PurgeJobsByMatch(t *testing.T) {
	orch := New(newMockPoller(nil), &mockSpawner{}, 50*time.Millisecond)

	job1 := JobInfo{ID: "JOB-1", Summary: "test fix login"}
	job2 := JobInfo{ID: "JOB-2", Summary: "other"}

	orch.completedJobs = append(orch.completedJobs, job1, job2)

	count, err := orch.PurgeJobsByMatch("login", silentLogger)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	completed := orch.GetCompletedJobs()
	assert.Len(t, completed, 1)
	assert.Equal(t, "JOB-2", completed[0].ID)
}
