package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockSpawnerForPause struct {
	mock.Mock
}

func (m *MockSpawnerForPause) Spawn(ctx context.Context, item WorkItem) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockSpawnerForPause) Cleanup(ctx context.Context, item WorkItem) error { return nil }

func (m *MockSpawnerForPause) Cancel(ctx context.Context, jobID string) error {
	args := m.Called(ctx, jobID)
	return args.Error(0)
}

func TestOrchestrator_PauseResumeConcurrencyGroup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))
	mockSpawner := new(MockSpawnerForPause)

	// Since job1 and job2 are in group1 which is paused, they should not be spawned.
	// Only job3 should be spawned.
	mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "job3"
	})).Return(nil)

	o := New(nil, mockSpawner, time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	item1 := WorkItem{ID: "job1", ConcurrencyGroup: "group1"}
	item2 := WorkItem{ID: "job2", ConcurrencyGroup: "group1"}
	item3 := WorkItem{ID: "job3", ConcurrencyGroup: "group2"}

	o.pendingJobs["job1"] = JobInfo{ID: "job1", WorkItem: item1, Status: "Pending"}
	o.pendingJobs["job2"] = JobInfo{ID: "job2", WorkItem: item2, Status: "Pending"}
	o.pendingJobs["job3"] = JobInfo{ID: "job3", WorkItem: item3, Status: "Pending"}

	o.PauseGroup("group1", logger)

	assert.NotNil(t, o.pausedGroups)
	assert.True(t, o.pausedGroups["group1"])

	o.evaluatePendingJobs(ctx, logger)

	// job1 and job2 should still be pending
	o.mu.RLock()
	_, ok1 := o.pendingJobs["job1"]
	_, ok2 := o.pendingJobs["job2"]
	_, ok3 := o.pendingJobs["job3"]
	o.mu.RUnlock()

	assert.True(t, ok1, "job1 should still be pending")
	assert.True(t, ok2, "job2 should still be pending")
	assert.False(t, ok3, "job3 should have been picked up")

	// Now resume group1
	o.ResumeGroup("group1", logger)
	assert.False(t, o.pausedGroups["group1"])

	mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "job1" || item.ID == "job2"
	})).Return(nil).Twice()

	o.evaluatePendingJobs(ctx, logger)

	time.Sleep(100 * time.Millisecond) // Give time for goroutines to process

	o.mu.RLock()
	_, ok1 = o.pendingJobs["job1"]
	_, ok2 = o.pendingJobs["job2"]
	o.mu.RUnlock()

	assert.False(t, ok1, "job1 should have been picked up")
	assert.False(t, ok2, "job2 should have been picked up")

	mockSpawner.AssertExpectations(t)
}
func (m *MockSpawnerForPause) GetLogs(ctx context.Context, jobID string) (io.ReadCloser, error) {
	args := m.Called(ctx, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}
func (m *MockSpawnerForPause) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
