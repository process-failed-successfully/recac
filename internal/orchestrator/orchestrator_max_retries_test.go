package orchestrator

import (
	"fmt"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPersistence struct {
	savedJobs map[string]JobInfo
}

func (m *mockPersistence) Init() error { return nil }
func (m *mockPersistence) SaveJob(job JobInfo) error {
	if m.savedJobs == nil {
		m.savedJobs = make(map[string]JobInfo)
	}
	m.savedJobs[job.ID] = job
	return nil
}
func (m *mockPersistence) GetJob(id string) (*JobInfo, error) {
	if m.savedJobs != nil {
		if job, ok := m.savedJobs[id]; ok {
			return &job, nil
		}
	}
	return nil, fmt.Errorf("job %s not found", id)
}
func (m *mockPersistence) GetJobs(limit int) ([]JobInfo, error) { return nil, nil }
func (m *mockPersistence) ClearHistory() (int, error) { return 0, nil }
func (m *mockPersistence) PurgeJob(id string) error { return nil }
func (m *mockPersistence) Close() error { return nil }

func TestUpdateJobMaxRetries(t *testing.T) {
	poller := &mockPoller{}
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 0)

	mp := &mockPersistence{}
	orch.SetPersistence(mp)

	ctx := context.Background()

	// 1. Job pending dependencies
	orch.mu.Lock()
	orch.pendingJobs["JOB-PENDING"] = JobInfo{
		ID:       "JOB-PENDING",
		WorkItem: WorkItem{ID: "JOB-PENDING", DependsOn: []string{"DEP"}}, // Has unmet dep so it stays pending
	}
	// 2. Active job
	orch.activeJobs["JOB-ACTIVE"] = JobInfo{
		ID:       "JOB-ACTIVE",
		WorkItem: WorkItem{ID: "JOB-ACTIVE"},
	}
	// 3. History job
	orch.completedJobs = append(orch.completedJobs, JobInfo{
		ID:       "JOB-HISTORY",
		WorkItem: WorkItem{ID: "JOB-HISTORY"},
	})
	orch.mu.Unlock()

	maxRetries := 3

	// Test pending job update
	err := orch.UpdateJobMaxRetries(ctx, "JOB-PENDING", maxRetries, nil)
	require.NoError(t, err)

	orch.mu.Lock()
	job, ok := orch.pendingJobs["JOB-PENDING"]
	orch.mu.Unlock()
	require.True(t, ok)
	require.NotNil(t, job.WorkItem.MaxRetries)
	assert.Equal(t, maxRetries, *job.WorkItem.MaxRetries)

	require.Contains(t, mp.savedJobs, "JOB-PENDING")
	assert.Equal(t, maxRetries, *mp.savedJobs["JOB-PENDING"].WorkItem.MaxRetries)

	// Test active job update
	err = orch.UpdateJobMaxRetries(ctx, "JOB-ACTIVE", maxRetries, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already active")

	// Test history job update
	err = orch.UpdateJobMaxRetries(ctx, "JOB-HISTORY", maxRetries, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already completed")

	// Test non-existent job
	err = orch.UpdateJobMaxRetries(ctx, "NON-EXISTENT", maxRetries, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUpdateJobsMaxRetriesBulk(t *testing.T) {
	poller := &mockPoller{}
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 0)

	mp := &mockPersistence{}
	orch.SetPersistence(mp)

	ctx := context.Background()

	orch.mu.Lock()
	orch.pendingJobs["JOB-1"] = JobInfo{
		ID: "JOB-1",
			Summary: "Fix bug 1",
		WorkItem: WorkItem{
			ID:        "JOB-1",
			Tags:      []string{"backend"},
			Summary:   "Fix bug 1",
			DependsOn: []string{"DEP"}, // Needs dep to stay pending
		},
	}
	orch.pendingJobs["JOB-2"] = JobInfo{
		ID: "JOB-2",
			Summary: "Fix bug 2",
		WorkItem: WorkItem{
			ID:        "JOB-2",
			Tags:      []string{"frontend"},
			Summary:   "Fix bug 2",
			DependsOn: []string{"DEP"},
		},
	}
	orch.pendingJobs["JOB-3"] = JobInfo{
		ID: "JOB-3",
			Summary: "New feature",
		WorkItem: WorkItem{
			ID:        "JOB-3",
			Tags:      []string{"backend", "urgent"},
			Summary:   "New feature",
			DependsOn: []string{"DEP"},
		},
	}
	orch.mu.Unlock()

	maxRetries := 5

	// Test UpdateJobsMaxRetriesByTag
	count, err := orch.UpdateJobsMaxRetriesByTag(ctx, "backend", maxRetries, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	orch.mu.Lock()
	j1, ok1 := orch.pendingJobs["JOB-1"]
	j2, ok2 := orch.pendingJobs["JOB-2"]
	j3, ok3 := orch.pendingJobs["JOB-3"]
	orch.mu.Unlock()

	require.True(t, ok1)
	require.True(t, ok2)
	require.True(t, ok3)

	require.NotNil(t, j1.WorkItem.MaxRetries)
	assert.Equal(t, maxRetries, *j1.WorkItem.MaxRetries)
	assert.Nil(t, j2.WorkItem.MaxRetries)
	require.NotNil(t, j3.WorkItem.MaxRetries)
	assert.Equal(t, maxRetries, *j3.WorkItem.MaxRetries)

	require.Contains(t, mp.savedJobs, "JOB-1")
	require.Contains(t, mp.savedJobs, "JOB-3")
	assert.NotContains(t, mp.savedJobs, "JOB-2")

	// Test UpdateJobsMaxRetriesByMatch
	maxRetriesMatch := 10
	countMatch, errMatch := orch.UpdateJobsMaxRetriesByMatch(ctx, "Fix", maxRetriesMatch, nil)
	require.NoError(t, errMatch)
	assert.Equal(t, 2, countMatch)

	orch.mu.Lock()
	j1, _ = orch.pendingJobs["JOB-1"]
	j2, _ = orch.pendingJobs["JOB-2"]
	j3, _ = orch.pendingJobs["JOB-3"]
	orch.mu.Unlock()

	require.NotNil(t, j1.WorkItem.MaxRetries)
	assert.Equal(t, maxRetriesMatch, *j1.WorkItem.MaxRetries)
	require.NotNil(t, j2.WorkItem.MaxRetries)
	assert.Equal(t, maxRetriesMatch, *j2.WorkItem.MaxRetries)
	require.NotNil(t, j3.WorkItem.MaxRetries)
	assert.Equal(t, maxRetries, *j3.WorkItem.MaxRetries) // Should not change

	require.Contains(t, mp.savedJobs, "JOB-1")
	require.Contains(t, mp.savedJobs, "JOB-2")

	// Test UpdateJobsMaxRetriesByMatch invalid regex
	countInvalid, errInvalid := orch.UpdateJobsMaxRetriesByMatch(ctx, "[invalid", 2, nil)
	require.Error(t, errInvalid)
	assert.Equal(t, 0, countInvalid)

	// Test UpdateJobsMaxRetriesByMatch no match
	countNoMatch, errNoMatch := orch.UpdateJobsMaxRetriesByMatch(ctx, "Nonexistent", 2, nil)
	require.NoError(t, errNoMatch)
	assert.Equal(t, 0, countNoMatch)
}
