package orchestrator

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestOrchestrator_JobTracking tests that active jobs are correctly tracked
func TestOrchestrator_JobTracking(t *testing.T) {
	// 1. Setup
	item := WorkItem{ID: "JOB-123", Summary: "Fix bug"}
	poller := newMockPoller([]WorkItem{item})

	blockCh := make(chan struct{})
	spawner := &blockingSpawner{blockCh: blockCh}

	interval := 100 * time.Millisecond
	orch := New(poller, spawner, interval)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. Start Orchestrator
	go func() {
		orch.Run(ctx, silentLogger)
	}()

	// 3. Wait for poll and spawn (blocking)
	// Give it enough time to poll and start the goroutine
	time.Sleep(150 * time.Millisecond)

	// 4. Verify Job is Active
	jobs := orch.GetActiveJobs()
	assert.Len(t, jobs, 1, "Should have 1 active job")
	if len(jobs) > 0 {
		assert.Equal(t, "JOB-123", jobs[0].ID)
		assert.Equal(t, "Fix bug", jobs[0].Summary)
		assert.Equal(t, "Spawning", jobs[0].Status)
	}

	// 5. Unblock Spawner
	close(blockCh)

	// 6. Wait for completion
	time.Sleep(50 * time.Millisecond)

	// 7. Verify Job is Gone
	jobs = orch.GetActiveJobs()
	assert.Len(t, jobs, 0, "Should have 0 active jobs after completion")
}

func TestOrchestrator_JobTimeout(t *testing.T) {
	// 1. Setup
	item := WorkItem{ID: "JOB-456", Summary: "Slow task"}
	poller := newMockPoller([]WorkItem{item})

	// Use a mock spawner that returns an error when context expires
	spawner := &timeoutSpawner{}

	interval := 10 * time.Millisecond
	orch := New(poller, spawner, interval)
	orch.JobTimeout = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. Start Orchestrator
	go func() {
		orch.Run(ctx, silentLogger)
	}()

	// 3. Wait for spawn and timeout
	// 10ms for interval, 50ms for timeout, so 100ms is plenty
	time.Sleep(100 * time.Millisecond)

	// 4. Verify Job timed out and moved to history
	jobs := orch.GetActiveJobs()
	assert.Len(t, jobs, 0, "Should have 0 active jobs due to timeout")

	history := orch.GetCompletedJobs()
	assert.Len(t, history, 1, "Should have 1 completed job in history")
	if len(history) > 0 {
		assert.Equal(t, "JOB-456", history[0].ID)
		assert.Equal(t, "Failed", history[0].Status)
		assert.Contains(t, history[0].Error, "context deadline exceeded")
	}
}

type timeoutSpawner struct{}

func (s *timeoutSpawner) Spawn(ctx context.Context, item WorkItem) error {
	<-ctx.Done()
	return ctx.Err()
}
func (s *timeoutSpawner) Cleanup(ctx context.Context, item WorkItem) error { return nil }
func (s *timeoutSpawner) Cancel(ctx context.Context, jobID string) error { return nil }
func (s *timeoutSpawner) Ping(ctx context.Context) error { return nil }
func (s *timeoutSpawner) GetLogs(ctx context.Context, jobID string) (io.ReadCloser, error) { return nil, nil }

func TestOrchestrator_JobDependencies(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)

	mockPoller.On("Poll", mock.Anything, mock.Anything).Return([]WorkItem{}, nil)

	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Submit Job A and Job B (B depends on A)
	jobA := WorkItem{ID: "JOB-A"}
	jobB := WorkItem{ID: "JOB-B", DependsOn: []string{"JOB-A"}}

	mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-A"
	})).Return(nil).Run(func(args mock.Arguments) {
		// Simulate Job A taking some time, then finishing
		time.Sleep(50 * time.Millisecond)
	}).Once()

	// B should only be spawned AFTER A finishes
	mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-B"
	})).Return(nil).Run(func(args mock.Arguments) {
	}).Once()

	go orch.Run(ctx, logger)
	time.Sleep(10 * time.Millisecond) // Let it start

	err := orch.SubmitJob(ctx, jobA, logger)
	assert.NoError(t, err)

	err = orch.SubmitJob(ctx, jobB, logger)
	assert.NoError(t, err)

	// B should be pending initially
	jobBInfo, err := orch.GetJob("JOB-B")
	assert.NoError(t, err)
	assert.Equal(t, "Pending", jobBInfo.Status)

	// Wait for B to complete
	time.Sleep(200 * time.Millisecond)

	jobBInfo, err = orch.GetJob("JOB-B")
	assert.NoError(t, err)
	assert.Equal(t, "Completed", jobBInfo.Status)

	mockSpawner.AssertExpectations(t)
}

func TestOrchestrator_JobDependenciesFailed(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)

	mockPoller.On("Poll", mock.Anything, mock.Anything).Return([]WorkItem{}, nil)

	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Submit Job A and Job B (B depends on A)
	jobA := WorkItem{ID: "JOB-A-FAIL"}
	jobB := WorkItem{ID: "JOB-B-FAIL", DependsOn: []string{"JOB-A-FAIL"}}

	mockPoller.On("UpdateStatus", mock.Anything, mock.Anything, "Failed", mock.Anything).Return(nil).Once()

	// A fails
	mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-A-FAIL"
	})).Return(fmt.Errorf("fatal error")).Once()

	// B should NEVER be spawned

	go orch.Run(ctx, logger)
	time.Sleep(10 * time.Millisecond) // Let it start

	err := orch.SubmitJob(ctx, jobA, logger)
	assert.NoError(t, err)

	err = orch.SubmitJob(ctx, jobB, logger)
	assert.NoError(t, err)

	// Wait for processing
	time.Sleep(150 * time.Millisecond)

	jobAInfo, err := orch.GetJob("JOB-A-FAIL")
	assert.NoError(t, err)
	assert.Equal(t, "Failed", jobAInfo.Status)

	jobBInfo, err := orch.GetJob("JOB-B-FAIL")
	assert.NoError(t, err)
	assert.Equal(t, "Failed", jobBInfo.Status)
	assert.Contains(t, jobBInfo.Error, "Dependency JOB-A-FAIL failed")

	mockSpawner.AssertExpectations(t)
}

func TestOrchestrator_CancelPendingJob(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)

	mockPoller.On("Poll", mock.Anything, mock.Anything).Return([]WorkItem{}, nil)

	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Submit Job A and Job B (B depends on A)
	jobA := WorkItem{ID: "JOB-A"}
	jobB := WorkItem{ID: "JOB-B", DependsOn: []string{"JOB-A"}}

	mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-A"
	})).Return(nil).Run(func(args mock.Arguments) {
		time.Sleep(100 * time.Millisecond)
	}).Once()

	go orch.Run(ctx, logger)
	time.Sleep(10 * time.Millisecond)

	err := orch.SubmitJob(ctx, jobA, logger)
	assert.NoError(t, err)

	err = orch.SubmitJob(ctx, jobB, logger)
	assert.NoError(t, err)

	jobBInfo, err := orch.GetJob("JOB-B")
	assert.NoError(t, err)
	assert.Equal(t, "Pending", jobBInfo.Status)

	// Cancel JOB-B while it's pending
	err = orch.CancelJob(ctx, "JOB-B")
	assert.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	// Verify JOB-B is now Canceled
	jobBInfo, err = orch.GetJob("JOB-B")
	assert.NoError(t, err)
	assert.Equal(t, "Canceled", jobBInfo.Status)

	mockSpawner.AssertExpectations(t)
}

func TestOrchestrator_CancelAllPendingJobs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)

	mockPoller.On("Poll", mock.Anything, mock.Anything).Return([]WorkItem{}, nil)

	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Submit Job A and Job B (B depends on A)
	jobA := WorkItem{ID: "JOB-A"}
	jobB := WorkItem{ID: "JOB-B", DependsOn: []string{"JOB-A"}}

	mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-A"
	})).Return(nil).Run(func(args mock.Arguments) {
		time.Sleep(100 * time.Millisecond)
	}).Once()

	// mockSpawner.Cancel for JOB-A
	mockSpawner.On("Cancel", mock.Anything, "JOB-A").Return(nil).Once()

	go orch.Run(ctx, logger)
	time.Sleep(10 * time.Millisecond)

	err := orch.SubmitJob(ctx, jobA, logger)
	assert.NoError(t, err)

	err = orch.SubmitJob(ctx, jobB, logger)
	assert.NoError(t, err)

	jobBInfo, err := orch.GetJob("JOB-B")
	assert.NoError(t, err)
	assert.Equal(t, "Pending", jobBInfo.Status)

	// Cancel all jobs
	count, err := orch.CancelAllJobs(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	time.Sleep(10 * time.Millisecond)

	// Verify JOB-B is now Canceled
	jobBInfo, err = orch.GetJob("JOB-B")
	assert.NoError(t, err)
	assert.Equal(t, "Canceled", jobBInfo.Status)

	mockSpawner.AssertExpectations(t)
}

func TestOrchestrator_JobSpecificTimeout(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)

	items := []WorkItem{
		{ID: "JOB-NO-TIMEOUT", Summary: "Fast job"},
		{ID: "JOB-TIMEOUT", Summary: "Slow job", Timeout: 10 * time.Millisecond},
	}

	mockPoller.On("Poll", mock.Anything, mock.Anything).Return(items, nil).Once()
	mockPoller.On("Poll", mock.Anything, mock.Anything).Return([]WorkItem{}, nil)
	mockPoller.On("UpdateStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-NO-TIMEOUT"
	})).Return(nil)

	mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-TIMEOUT"
	})).Run(func(args mock.Arguments) {
		ctx := args.Get(0).(context.Context)
		<-ctx.Done()
	}).Return(context.DeadlineExceeded)

	orch := New(mockPoller, mockSpawner, 10*time.Millisecond)
	orch.JobTimeout = 1 * time.Hour

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go orch.Run(ctx, logger)

	time.Sleep(100 * time.Millisecond)
	cancel()

	orch.mu.RLock()
	defer orch.mu.RUnlock()

	var jobNoTimeout, jobTimeout JobInfo
	for _, j := range orch.completedJobs {
		if j.ID == "JOB-NO-TIMEOUT" {
			jobNoTimeout = j
		}
		if j.ID == "JOB-TIMEOUT" {
			jobTimeout = j
		}
	}

	assert.Equal(t, "Completed", jobNoTimeout.Status)
	assert.Equal(t, "Failed", jobTimeout.Status)
	assert.Contains(t, jobTimeout.Error, "context deadline exceeded")
}
