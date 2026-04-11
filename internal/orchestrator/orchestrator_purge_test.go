package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestrator_PurgeJobBasic(t *testing.T) {
	orch := New(newMockPoller(nil), &mockSpawner{}, 50*time.Millisecond)

	job1 := JobInfo{ID: "JOB-1", Status: "Failed"}
	orch.completedJobs = append(orch.completedJobs, job1)

	// Purge active job shouldn't do anything because the function works on completed jobs
	err := orch.PurgeJob("JOB-1", silentLogger)
	assert.NoError(t, err)

	completed := orch.GetCompletedJobs()
	assert.Len(t, completed, 0)
}

func TestOrchestrator_PurgeJobNotFound(t *testing.T) {
	orch := New(newMockPoller(nil), &mockSpawner{}, 50*time.Millisecond)
	err := orch.PurgeJob("JOB-X", silentLogger)
	assert.Error(t, err)
}

func TestOrchestrator_PurgeJobActive(t *testing.T) {
	orch := New(newMockPoller(nil), &mockSpawner{
		blockCh: make(chan struct{}),
	}, 50*time.Millisecond)

	err := orch.SubmitJob(context.Background(), WorkItem{ID: "JOB-1"}, silentLogger)
	require.NoError(t, err)

	err = orch.PurgeJob("JOB-1", silentLogger)
	assert.Error(t, err)
}

func TestOrchestrator_PurgeJobsByStatus(t *testing.T) {
	orch := New(newMockPoller(nil), &mockSpawner{}, 50*time.Millisecond)

	mockPersistence := &mockPersistenceClear{}
	orch.SetPersistence(mockPersistence)

	job1 := JobInfo{ID: "JOB-1", Status: "Failed"}
	job2 := JobInfo{ID: "JOB-2", Status: "Completed"}

	orch.completedJobs = append(orch.completedJobs, job1, job2)

	// Add pending job
	orch.pendingJobs["JOB-3"] = JobInfo{ID: "JOB-3", Status: "Failed"}

	// Add active job
	orch.activeJobs["JOB-4"] = JobInfo{ID: "JOB-4", Status: "Failed"}

	dbJobs := []JobInfo{
		{ID: "JOB-5", Status: "Failed"},
		{ID: "JOB-6", Status: "Completed"},
	}
	mockPersistence.On("GetJobs", 10000).Return(dbJobs, nil)
	mockPersistence.On("PurgeJob", "JOB-5").Return(nil)

	count, err := orch.PurgeJobsByStatus("Failed", silentLogger)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	completed := orch.GetCompletedJobs()
	assert.Len(t, completed, 1)
	assert.Equal(t, "JOB-2", completed[0].ID)

	mockPersistence.AssertExpectations(t)
}

func TestOrchestrator_PurgeJobsOlderThan(t *testing.T) {
	orch := New(newMockPoller(nil), &mockSpawner{}, 50*time.Millisecond)

	mockPersistence := &mockPersistenceClear{}
	orch.SetPersistence(mockPersistence)

	now := time.Now()

	job1 := JobInfo{ID: "JOB-1", StartTime: now.Add(-30 * time.Hour), EndTime: now.Add(-29 * time.Hour)}
	job2 := JobInfo{ID: "JOB-2", StartTime: now.Add(-10 * time.Hour), EndTime: now.Add(-9 * time.Hour)}
	job3 := JobInfo{ID: "JOB-3", StartTime: now.Add(-48 * time.Hour)} // No EndTime

	orch.completedJobs = append(orch.completedJobs, job1, job2, job3)

	dbJobs := []JobInfo{
		{ID: "JOB-4", StartTime: now.Add(-5 * time.Hour), EndTime: now.Add(-4 * time.Hour)},
		{ID: "JOB-5", StartTime: now.Add(-100 * time.Hour), EndTime: now.Add(-99 * time.Hour)},
	}
	mockPersistence.On("GetJobs", 10000).Return(dbJobs, nil)
	mockPersistence.On("PurgeJob", "JOB-5").Return(nil)

	// Purge jobs older than 24 hours
	count, err := orch.PurgeJobsOlderThan(24*time.Hour, silentLogger)
	assert.NoError(t, err)
	assert.Equal(t, 3, count) // JOB-1, JOB-3, JOB-5

	completed := orch.GetCompletedJobs()
	assert.Len(t, completed, 1)
	assert.Equal(t, "JOB-2", completed[0].ID)

	mockPersistence.AssertExpectations(t)
}

func TestOrchestrator_PurgeJobsByGroup(t *testing.T) {
	orch := New(newMockPoller(nil), &mockSpawner{}, 50*time.Millisecond)

	mockPersistence := &mockPersistenceClear{}
	orch.SetPersistence(mockPersistence)

	job1 := JobInfo{ID: "JOB-1", WorkItem: WorkItem{ConcurrencyGroup: "test-group"}}
	job2 := JobInfo{ID: "JOB-2", WorkItem: WorkItem{ConcurrencyGroup: "other"}}

	orch.completedJobs = append(orch.completedJobs, job1, job2)

	// Add pending job
	orch.pendingJobs["JOB-3"] = JobInfo{ID: "JOB-3", WorkItem: WorkItem{ConcurrencyGroup: "test-group"}}

	// Add active job
	orch.activeJobs["JOB-4"] = JobInfo{ID: "JOB-4", WorkItem: WorkItem{ConcurrencyGroup: "test-group"}}

	dbJobs := []JobInfo{
		{ID: "JOB-5", WorkItem: WorkItem{ConcurrencyGroup: "test-group"}},
		{ID: "JOB-6", WorkItem: WorkItem{ConcurrencyGroup: "do not delete"}},
	}
	mockPersistence.On("GetJobs", 10000).Return(dbJobs, nil)
	mockPersistence.On("PurgeJob", "JOB-5").Return(nil)

	count, err := orch.PurgeJobsByGroup("test-group", silentLogger)
	assert.NoError(t, err)
	assert.Equal(t, 2, count) // JOB-1 (memory) and JOB-5 (db)

	completed := orch.GetCompletedJobs()
	assert.Len(t, completed, 1)
	assert.Equal(t, "JOB-2", completed[0].ID)

	// Pending and Active jobs should not be purged
	assert.Contains(t, orch.pendingJobs, "JOB-3")
	assert.Contains(t, orch.activeJobs, "JOB-4")

	mockPersistence.AssertExpectations(t)
}

func TestOrchestrator_PurgeJobsByMatch(t *testing.T) {
	orch := New(newMockPoller(nil), &mockSpawner{}, 50*time.Millisecond)

	mockPersistence := &mockPersistenceClear{}
	orch.SetPersistence(mockPersistence)

	job1 := JobInfo{ID: "JOB-1", Summary: "test fix login"}
	job2 := JobInfo{ID: "JOB-2", Summary: "other"}

	orch.completedJobs = append(orch.completedJobs, job1, job2)

	// Add pending job
	orch.pendingJobs["JOB-3"] = JobInfo{ID: "JOB-3", Summary: "test fix login"}

	// Add active job
	orch.activeJobs["JOB-4"] = JobInfo{ID: "JOB-4", Summary: "test fix login"}

	dbJobs := []JobInfo{
		{ID: "JOB-5", Summary: "fix login issue"},
		{ID: "JOB-6", Summary: "do not delete"},
	}
	mockPersistence.On("GetJobs", 10000).Return(dbJobs, nil)
	mockPersistence.On("PurgeJob", "JOB-5").Return(nil)

	count, err := orch.PurgeJobsByMatch("login", silentLogger)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	completed := orch.GetCompletedJobs()
	assert.Len(t, completed, 1)
	assert.Equal(t, "JOB-2", completed[0].ID)

	_, err = orch.PurgeJobsByMatch("[invalid regex", silentLogger)
	assert.Error(t, err)

	mockPersistence.AssertExpectations(t)
}
