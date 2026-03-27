package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestOrchestrator_ConcurrencyGroup(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-1"
	})).Return(nil).Run(func(args mock.Arguments) {
		time.Sleep(500 * time.Millisecond)
	})
	mockSpawner.On("Cancel", mock.Anything, "JOB-1").Return(nil)
	mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-2"
	})).Return(nil)

	err := orch.SubmitJob(ctx, WorkItem{
		ID:               "JOB-1",
		Summary:          "First job",
		ConcurrencyGroup: "group-A",
		CancelInProgress: true,
	}, logger)
	assert.NoError(t, err)

	activeJobs := orch.GetActiveJobs()
	assert.Len(t, activeJobs, 1)

	err = orch.SubmitJob(ctx, WorkItem{
		ID:               "JOB-2",
		Summary:          "Second job",
		ConcurrencyGroup: "group-A",
		CancelInProgress: true,
	}, logger)
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	mockSpawner.AssertCalled(t, "Cancel", mock.Anything, "JOB-1")
}

func TestOrchestrator_ConcurrencyGroup_Delayed(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)
    mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-1"
	})).Return(nil).Run(func(args mock.Arguments) {
		time.Sleep(500 * time.Millisecond)
	})
    mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-2"
	})).Return(nil)
    mockSpawner.On("Cancel", mock.Anything, "JOB-1").Return(nil)

	err := orch.SubmitJob(ctx, WorkItem{
		ID:               "JOB-1",
		Summary:          "First job",
		ConcurrencyGroup: "group-B",
		CancelInProgress: true,
	}, logger)
	assert.NoError(t, err)

	err = orch.SubmitJob(ctx, WorkItem{
		ID:               "JOB-2",
		Summary:          "Second job",
		ConcurrencyGroup: "group-B",
		CancelInProgress: true,
		Delay:            1 * time.Hour,
	}, logger)
	assert.NoError(t, err)
    time.Sleep(50 * time.Millisecond)
    mockSpawner.AssertCalled(t, "Cancel", mock.Anything, "JOB-1")
}

func TestOrchestrator_ConcurrencyGroup_Held(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)
    mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-1"
	})).Return(nil).Run(func(args mock.Arguments) {
		time.Sleep(500 * time.Millisecond)
	})
    mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-2"
	})).Return(nil)
    mockSpawner.On("Cancel", mock.Anything, "JOB-1").Return(nil)

	err := orch.SubmitJob(ctx, WorkItem{
		ID:               "JOB-1",
		Summary:          "First job",
		ConcurrencyGroup: "group-C",
		CancelInProgress: true,
	}, logger)
	assert.NoError(t, err)

	err = orch.SubmitJob(ctx, WorkItem{
		ID:               "JOB-2",
		Summary:          "Second job",
		ConcurrencyGroup: "group-C",
		CancelInProgress: true,
		Hold:             true,
	}, logger)
	assert.NoError(t, err)
    time.Sleep(50 * time.Millisecond)
    mockSpawner.AssertCalled(t, "Cancel", mock.Anything, "JOB-1")
}

func TestOrchestrator_ConcurrencyGroup_PendingApproval(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)
    mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-1"
	})).Return(nil).Run(func(args mock.Arguments) {
		time.Sleep(500 * time.Millisecond)
	})
    mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-2"
	})).Return(nil)
    mockSpawner.On("Cancel", mock.Anything, "JOB-1").Return(nil)

    tVal := true
	err := orch.SubmitJob(ctx, WorkItem{
		ID:               "JOB-1",
		Summary:          "First job",
		ConcurrencyGroup: "group-D",
		CancelInProgress: true,
	}, logger)
	assert.NoError(t, err)

	err = orch.SubmitJob(ctx, WorkItem{
		ID:               "JOB-2",
		Summary:          "Second job",
		ConcurrencyGroup: "group-D",
		CancelInProgress: true,
		RequireApproval:  &tVal,
	}, logger)
	assert.NoError(t, err)
    time.Sleep(50 * time.Millisecond)
    mockSpawner.AssertCalled(t, "Cancel", mock.Anything, "JOB-1")
}

func TestOrchestrator_ConcurrencyGroup_PendingDep(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)
    mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-1"
	})).Return(nil).Run(func(args mock.Arguments) {
		time.Sleep(500 * time.Millisecond)
	})
    mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-0"
	})).Return(nil)
    mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-2"
	})).Return(nil)
    mockSpawner.On("Cancel", mock.Anything, "JOB-1").Return(nil)

	err := orch.SubmitJob(ctx, WorkItem{
		ID:               "JOB-1",
		Summary:          "First job",
		ConcurrencyGroup: "group-E",
		CancelInProgress: true,
	}, logger)
	assert.NoError(t, err)

    err = orch.SubmitJob(ctx, WorkItem{
		ID:               "JOB-0",
		Summary:          "Job 0",
	}, logger)
    assert.NoError(t, err)

	err = orch.SubmitJob(ctx, WorkItem{
		ID:               "JOB-2",
		Summary:          "Second job",
		ConcurrencyGroup: "group-E",
		CancelInProgress: true,
		DependsOn:        []string{"JOB-0"},
	}, logger)
	assert.NoError(t, err)
    time.Sleep(50 * time.Millisecond)
    mockSpawner.AssertCalled(t, "Cancel", mock.Anything, "JOB-1")
}

func TestOrchestrator_ConcurrencyGroup_Failure(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)
    mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-1"
	})).Return(nil).Run(func(args mock.Arguments) {
		time.Sleep(500 * time.Millisecond)
	})
    mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-FAILED"
	})).Return(nil)
    mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-2"
	})).Return(nil)
    mockSpawner.On("Cancel", mock.Anything, "JOB-1").Return(nil)

	err := orch.SubmitJob(ctx, WorkItem{
		ID:               "JOB-1",
		Summary:          "First job",
		ConcurrencyGroup: "group-F",
		CancelInProgress: true,
	}, logger)
	assert.NoError(t, err)

    // forcefully fail JOB-FAILED
    orch.mu.Lock()
    orch.completedJobs = append(orch.completedJobs, JobInfo{
        ID: "JOB-FAILED",
        Status: "Failed",
    })
    orch.mu.Unlock()

	err = orch.SubmitJob(ctx, WorkItem{
		ID:               "JOB-2",
		Summary:          "Second job",
		ConcurrencyGroup: "group-F",
		CancelInProgress: true,
		DependsOn:        []string{"JOB-FAILED"},
	}, logger)
	assert.ErrorContains(t, err, "failed")
    time.Sleep(50 * time.Millisecond)
    mockSpawner.AssertCalled(t, "Cancel", mock.Anything, "JOB-1")
}

func TestOrchestrator_ConcurrencyGroup_Skipped(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)
    mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-1"
	})).Return(nil).Run(func(args mock.Arguments) {
		time.Sleep(500 * time.Millisecond)
	})
    mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-2"
	})).Return(nil)
    mockSpawner.On("Cancel", mock.Anything, "JOB-1").Return(nil)

	err := orch.SubmitJob(ctx, WorkItem{
		ID:               "JOB-1",
		Summary:          "First job",
		ConcurrencyGroup: "group-F",
		CancelInProgress: true,
	}, logger)
	assert.NoError(t, err)


	err = orch.SubmitJob(ctx, WorkItem{
		ID:               "JOB-2",
		Summary:          "Second job",
		ConcurrencyGroup: "group-F",
		CancelInProgress: true,
		RunCondition:     "false",
	}, logger)
	assert.NoError(t, err)
    time.Sleep(50 * time.Millisecond)
    mockSpawner.AssertCalled(t, "Cancel", mock.Anything, "JOB-1")
}

func TestOrchestrator_ConcurrencyGroup_Sequential(t *testing.T) {
	mockPoller := new(MockPoller)
	mockSpawner := new(MockSpawner)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	orch := New(mockPoller, mockSpawner, 100*time.Millisecond)

	spawnCompleteCh := make(chan struct{})

	// Setup Spawner mocks
	// First job will be spawned and block until channel is closed
	mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-1"
	})).Return(nil).Run(func(args mock.Arguments) {
		<-spawnCompleteCh
	})

	// Second job will be spawned later
	mockSpawner.On("Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-2"
	})).Return(nil)

	// Submit Job 1
	err := orch.SubmitJob(ctx, WorkItem{
		ID:               "JOB-1",
		Summary:          "First job",
		ConcurrencyGroup: "group-sequential",
		CancelInProgress: false,
	}, logger)
	assert.NoError(t, err)

	// Verify Job 1 is active
	activeJobs := orch.GetActiveJobs()
	assert.Len(t, activeJobs, 1)
	assert.Equal(t, "JOB-1", activeJobs[0].ID)

	// Submit Job 2 in the same concurrency group with CancelInProgress = false
	err = orch.SubmitJob(ctx, WorkItem{
		ID:               "JOB-2",
		Summary:          "Second job",
		ConcurrencyGroup: "group-sequential",
		CancelInProgress: false, // This is the crucial part
	}, logger)
	assert.NoError(t, err)

	// Verify Job 2 is pending, NOT active
	pendingJobs := orch.GetPendingJobs()
	assert.Len(t, pendingJobs, 1)
	assert.Equal(t, "JOB-2", pendingJobs[0].ID)

	// And verify active jobs includes both, but we can verify JOB-2 is in pending and JOB-1 is Spawning
	allActiveAndPending := orch.GetActiveJobs()
	assert.Len(t, allActiveAndPending, 2)

	// Ensure Cancel was NOT called for JOB-1
	mockSpawner.AssertNotCalled(t, "Cancel", mock.Anything, "JOB-1")

	// Finish Job 1
	close(spawnCompleteCh)

	// Wait for evaluatePendingJobs to pick up Job 2
	time.Sleep(150 * time.Millisecond)

	// Verify Job 2 is now active or completed (mock Spawn returns immediately)
	pendingJobs = orch.GetPendingJobs()
	assert.Len(t, pendingJobs, 0)

	completedJobs := orch.GetCompletedJobs()
	assert.Len(t, completedJobs, 2)

	// Verify Spawner was called for Job 2
	mockSpawner.AssertCalled(t, "Spawn", mock.Anything, mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "JOB-2"
	}))
}
