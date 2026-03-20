package orchestrator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetJobBlockers(t *testing.T) {
	mockPoller := &MockPoller{}
	mockSpawner := &MockSpawner{}
	orch := New(mockPoller, mockSpawner, 1*time.Second)

	// 1. Job not found
	_, err := orch.GetJobBlockers("NON_EXISTENT")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job NON_EXISTENT not found")

	// 2. Job with no dependencies
	orch.activeJobs["JOB_NO_DEPS"] = JobInfo{
		ID: "JOB_NO_DEPS",
		WorkItem: WorkItem{
			ID: "JOB_NO_DEPS",
		},
	}
	blockers, err := orch.GetJobBlockers("JOB_NO_DEPS")
	assert.NoError(t, err)
	assert.Empty(t, blockers)

	// 3. Job with missing dependency
	orch.activeJobs["JOB_MISSING_DEP"] = JobInfo{
		ID: "JOB_MISSING_DEP",
		WorkItem: WorkItem{
			ID: "JOB_MISSING_DEP",
			DependsOn: []string{"MISSING_DEP"},
		},
	}
	blockers, err = orch.GetJobBlockers("JOB_MISSING_DEP")
	assert.NoError(t, err)
	assert.Len(t, blockers, 1)
	assert.Equal(t, "Missing", blockers[0].Status)
	assert.Equal(t, "MISSING_DEP", blockers[0].ID)

	// 4. Job with completed dependency
	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:     "COMPLETED_DEP",
		Status: "Completed",
	})
	orch.activeJobs["JOB_COMPLETED_DEP"] = JobInfo{
		ID: "JOB_COMPLETED_DEP",
		WorkItem: WorkItem{
			ID: "JOB_COMPLETED_DEP",
			DependsOn: []string{"COMPLETED_DEP"},
		},
	}
	blockers, err = orch.GetJobBlockers("JOB_COMPLETED_DEP")
	assert.NoError(t, err)
	assert.Empty(t, blockers)

	// 5. Job with pending dependency
	orch.pendingJobs["PENDING_DEP"] = JobInfo{
		ID:     "PENDING_DEP",
		Status: "Pending",
	}
	orch.activeJobs["JOB_PENDING_DEP"] = JobInfo{
		ID: "JOB_PENDING_DEP",
		WorkItem: WorkItem{
			ID: "JOB_PENDING_DEP",
			DependsOn: []string{"PENDING_DEP"},
		},
	}
	blockers, err = orch.GetJobBlockers("JOB_PENDING_DEP")
	assert.NoError(t, err)
	assert.Len(t, blockers, 1)
	assert.Equal(t, "Pending", blockers[0].Status)
	assert.Equal(t, "PENDING_DEP", blockers[0].ID)

	// 6. Job with failed dependency (blocks default on_success)
	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:     "FAILED_DEP",
		Status: "Failed",
	})
	orch.activeJobs["JOB_FAILED_DEP"] = JobInfo{
		ID: "JOB_FAILED_DEP",
		WorkItem: WorkItem{
			ID: "JOB_FAILED_DEP",
			DependsOn: []string{"FAILED_DEP"},
		},
	}
	blockers, err = orch.GetJobBlockers("JOB_FAILED_DEP")
	assert.NoError(t, err)
	assert.Len(t, blockers, 1)
	assert.Equal(t, "Failed", blockers[0].Status)
	assert.Equal(t, "FAILED_DEP", blockers[0].ID)

	// 7. Job with failed dependency (does NOT block on_failure)
	orch.activeJobs["JOB_FAILED_DEP_ON_FAIL"] = JobInfo{
		ID: "JOB_FAILED_DEP_ON_FAIL",
		WorkItem: WorkItem{
			ID:           "JOB_FAILED_DEP_ON_FAIL",
			DependsOn:    []string{"FAILED_DEP"},
			RunCondition: "on_failure",
		},
	}
	blockers, err = orch.GetJobBlockers("JOB_FAILED_DEP_ON_FAIL")
	assert.NoError(t, err)
	assert.Empty(t, blockers)
}
