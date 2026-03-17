package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOrchestrator_DeletePendingJob(t *testing.T) {
	orch := New(nil, nil, time.Minute)

	item := WorkItem{
		ID:      "TEST-1",
		Summary: "Test Summary",
	}

	job := JobInfo{
		ID:       item.ID,
		Status:   "Pending",
		WorkItem: item,
	}

	orch.pendingJobs[item.ID] = job

	err := orch.DeletePendingJob(context.Background(), "TEST-1", nil)
	assert.NoError(t, err)

	_, exists := orch.pendingJobs["TEST-1"]
	assert.False(t, exists)
}

func TestOrchestrator_DeletePendingJob_NotFound(t *testing.T) {
	orch := New(nil, nil, time.Minute)

	err := orch.DeletePendingJob(context.Background(), "NON-EXISTENT", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in pending queue")
}

func TestOrchestrator_DeletePendingJob_Active(t *testing.T) {
	orch := New(nil, nil, time.Minute)

	item := WorkItem{
		ID:      "TEST-1",
		Summary: "Test Summary",
	}

	job := JobInfo{
		ID:       item.ID,
		Status:   "Active",
		WorkItem: item,
	}

	orch.activeJobs[item.ID] = job

	err := orch.DeletePendingJob(context.Background(), "TEST-1", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already active")
}

func TestOrchestrator_DeletePendingJobsByTag(t *testing.T) {
	orch := New(nil, nil, time.Minute)

	job1 := JobInfo{
		ID:       "TEST-1",
		Status:   "Pending",
		WorkItem: WorkItem{ID: "TEST-1", Tags: []string{"tag1", "tag2"}},
	}
	job2 := JobInfo{
		ID:       "TEST-2",
		Status:   "Pending",
		WorkItem: WorkItem{ID: "TEST-2", Tags: []string{"tag2"}},
	}
	job3 := JobInfo{
		ID:       "TEST-3",
		Status:   "Pending",
		WorkItem: WorkItem{ID: "TEST-3", Tags: []string{"tag3"}},
	}

	orch.pendingJobs["TEST-1"] = job1
	orch.pendingJobs["TEST-2"] = job2
	orch.pendingJobs["TEST-3"] = job3

	count, err := orch.DeletePendingJobsByTag(context.Background(), "tag2", nil)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	_, exists1 := orch.pendingJobs["TEST-1"]
	assert.False(t, exists1)
	_, exists2 := orch.pendingJobs["TEST-2"]
	assert.False(t, exists2)
	_, exists3 := orch.pendingJobs["TEST-3"]
	assert.True(t, exists3)
}

func TestOrchestrator_DeletePendingJobsByMatch(t *testing.T) {
	orch := New(nil, nil, time.Minute)

	job1 := JobInfo{
		ID:       "TEST-1",
		Summary:  "Fix bug in login",
		Status:   "Pending",
		WorkItem: WorkItem{ID: "TEST-1", Summary: "Fix bug in login"},
	}
	job2 := JobInfo{
		ID:       "TEST-2",
		Summary:  "Update UI",
		Status:   "Pending",
		WorkItem: WorkItem{ID: "TEST-2", Summary: "Update UI"},
	}
	job3 := JobInfo{
		ID:       "TEST-3",
		Summary:  "Fix bug in payment",
		Status:   "Pending",
		WorkItem: WorkItem{ID: "TEST-3", Summary: "Fix bug in payment"},
	}

	orch.pendingJobs["TEST-1"] = job1
	orch.pendingJobs["TEST-2"] = job2
	orch.pendingJobs["TEST-3"] = job3

	count, err := orch.DeletePendingJobsByMatch(context.Background(), "Fix bug", nil)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	_, exists1 := orch.pendingJobs["TEST-1"]
	assert.False(t, exists1)
	_, exists2 := orch.pendingJobs["TEST-2"]
	assert.True(t, exists2)
	_, exists3 := orch.pendingJobs["TEST-3"]
	assert.False(t, exists3)
}

func TestOrchestrator_DeletePendingJob_Completed(t *testing.T) {
	orch := New(nil, nil, time.Minute)

	orch.completedJobs = []JobInfo{{ID: "TEST-1"}}

	err := orch.DeletePendingJob(context.Background(), "TEST-1", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already completed")
}

func TestOrchestrator_DeletePendingJob_Timer(t *testing.T) {
	orch := New(nil, nil, time.Minute)

	orch.pendingJobs["TEST-1"] = JobInfo{ID: "TEST-1"}
	orch.delayTimers["TEST-1"] = time.AfterFunc(1*time.Hour, func() {})

	err := orch.DeletePendingJob(context.Background(), "TEST-1", nil)
	assert.NoError(t, err)

	_, ok := orch.pendingJobs["TEST-1"]
	assert.False(t, ok)
	_, timerOk := orch.delayTimers["TEST-1"]
	assert.False(t, timerOk)
}

func TestOrchestrator_DeletePendingJobsByMatch_InvalidRegex(t *testing.T) {
	orch := New(nil, nil, time.Minute)

	_, err := orch.DeletePendingJobsByMatch(context.Background(), "[invalid", nil)
	assert.Error(t, err)
}

func TestOrchestrator_DeletePendingJobsByMatch_Timer(t *testing.T) {
	orch := New(nil, nil, time.Minute)

	orch.pendingJobs["TEST-1"] = JobInfo{ID: "TEST-1", Summary: "match me"}
	orch.delayTimers["TEST-1"] = time.AfterFunc(1*time.Hour, func() {})

	count, err := orch.DeletePendingJobsByMatch(context.Background(), "match", nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	_, ok := orch.pendingJobs["TEST-1"]
	assert.False(t, ok)
	_, timerOk := orch.delayTimers["TEST-1"]
	assert.False(t, timerOk)
}

func TestOrchestrator_DeletePendingJobsByTag_Timer(t *testing.T) {
	orch := New(nil, nil, time.Minute)

	orch.pendingJobs["TEST-1"] = JobInfo{ID: "TEST-1", WorkItem: WorkItem{Tags: []string{"tag1"}}}
	orch.delayTimers["TEST-1"] = time.AfterFunc(1*time.Hour, func() {})

	count, err := orch.DeletePendingJobsByTag(context.Background(), "tag1", nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	_, ok := orch.pendingJobs["TEST-1"]
	assert.False(t, ok)
	_, timerOk := orch.delayTimers["TEST-1"]
	assert.False(t, timerOk)
}
