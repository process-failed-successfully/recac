package orchestrator

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestOrchestrator_CancelJobsByConcurrencyGroup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Cancel", mock.Anything, mock.Anything).Return(nil)

	o := New(nil, mockSpawner, time.Second)
	ctx := context.Background()

	o.pendingJobs["pending-1"] = JobInfo{
		ID:    "pending-1",
		Status: "pending",
		WorkItem: WorkItem{
			ConcurrencyGroup: "test-group",
		},
	}
	o.pendingJobs["pending-2"] = JobInfo{
		ID:    "pending-2",
		Status: "pending",
		WorkItem: WorkItem{
			ConcurrencyGroup: "other-group",
		},
	}
	o.activeJobs["active-1"] = JobInfo{
		ID:    "active-1",
		Status: "active",
		WorkItem: WorkItem{
			ConcurrencyGroup: "test-group",
		},
	}

	count, err := o.CancelJobsByConcurrencyGroup(ctx, "test-group", logger)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestOrchestrator_CancelJobsByConcurrencyGroup_Error(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Cancel", mock.Anything, mock.Anything).Return(assert.AnError)

	o := New(nil, mockSpawner, time.Second)
	ctx := context.Background()

	o.activeJobs["active-1"] = JobInfo{
		ID:    "active-1",
		Status: "active",
		WorkItem: WorkItem{
			ConcurrencyGroup: "test-group",
		},
	}

	count, err := o.CancelJobsByConcurrencyGroup(ctx, "test-group", logger)
	assert.Error(t, err)
	assert.Equal(t, 0, count)
}

func TestOrchestrator_SubmitJobWithConcurrencyGroupCancel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Cancel", mock.Anything, mock.Anything).Return(nil)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)

	o := New(nil, mockSpawner, time.Second)

	ctx := context.Background()

	o.pendingJobs["pending-cg"] = JobInfo{
		ID:    "pending-cg",
		Status: "pending",
		WorkItem: WorkItem{
			ConcurrencyGroup: "test-cg",
		},
	}

	newItem := WorkItem{
		ID:               "new-cg-job",
		ConcurrencyGroup: "test-cg",
		CancelInProgress: true,
	}

	err := o.SubmitJob(ctx, newItem, logger)
	assert.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
}

func TestOrchestrator_ProcessWorkItemInternal_ConcurrencyGroupActive(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)

	o := New(nil, mockSpawner, time.Second)
	ctx := context.Background()

	o.activeJobs["active-cg"] = JobInfo{
		ID:    "active-cg",
		Status: "active",
		WorkItem: WorkItem{
			ConcurrencyGroup: "test-cg-active",
		},
	}

	newItem := WorkItem{
		ID:               "new-cg-job-2",
		ConcurrencyGroup: "test-cg-active",
		CancelInProgress: false,
	}

	err := o.SubmitJob(ctx, newItem, logger)
	assert.NoError(t, err)
}

func TestOrchestrator_ProcessWorkItemInternal_Draining(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)

	o := New(nil, mockSpawner, time.Second)
	o.Drain(logger)
	ctx := context.Background()

	newItem := WorkItem{
		ID: "new-job-draining",
	}

	err := o.SubmitJob(ctx, newItem, logger)
	assert.Error(t, err)
	assert.Equal(t, ErrDraining, err)
}

func TestOrchestrator_ProcessWorkItemInternal_AlreadyPendingApproval(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockSpawner := new(MockSpawner)

	o := New(nil, mockSpawner, time.Second)
	ctx := context.Background()

	o.pendingJobs["pending-approval-job"] = JobInfo{
		ID:     "pending-approval-job",
		Status: "Pending Approval",
	}

	newItem := WorkItem{
		ID: "pending-approval-job",
	}

	err := o.SubmitJob(ctx, newItem, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is already pending approval")
}

func TestOrchestrator_ProcessWorkItemInternal_AlreadyPendingDependencies(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockSpawner := new(MockSpawner)

	o := New(nil, mockSpawner, time.Second)
	ctx := context.Background()

	o.pendingJobs["pending-dep-job"] = JobInfo{
		ID:     "pending-dep-job",
		Status: "Pending dependencies",
	}

	newItem := WorkItem{
		ID: "pending-dep-job",
	}

	err := o.SubmitJob(ctx, newItem, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is already pending dependencies")
}

func TestOrchestrator_ProcessWorkItemInternal_ConcurrencyGroupCancelActive(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Cancel", mock.Anything, mock.Anything).Return(nil)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)

	o := New(nil, mockSpawner, time.Second)
	ctx := context.Background()

	o.activeJobs["active-cg-cancel"] = JobInfo{
		ID:    "active-cg-cancel",
		Status: "active",
		WorkItem: WorkItem{
			ConcurrencyGroup: "test-cg-active-cancel",
		},
	}

	newItem := WorkItem{
		ID:               "new-cg-job-3",
		ConcurrencyGroup: "test-cg-active-cancel",
		CancelInProgress: true,
	}

	err := o.SubmitJob(ctx, newItem, logger)
	assert.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
}

func TestOrchestrator_ProcessWorkItemInternal_RetryCount(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)

	o := New(nil, mockSpawner, time.Second)
	ctx := context.Background()

	o.activeJobs["test-cg-retry-active"] = JobInfo{
		ID:     "test-cg-retry-active",
		Status: "active",
		WorkItem: WorkItem{
			ConcurrencyGroup: "test-cg-retry",
		},
	}

	newItem := WorkItem{
		ID:               "new-cg-job-retry",
		ConcurrencyGroup: "test-cg-retry",
		CancelInProgress: false,
	}

	err := o.processWorkItemInternal(ctx, newItem, 1, false, logger)
	assert.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
}

func TestOrchestrator_ProcessWorkItemInternal_RunAfterCancel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Cancel", mock.Anything, mock.Anything).Return(nil)

	o := New(nil, mockSpawner, time.Second)
	ctx := context.Background()

	o.activeJobs["active-cg-runafter"] = JobInfo{
		ID:     "active-cg-runafter",
		Status: "active",
		WorkItem: WorkItem{
			ConcurrencyGroup: "test-cg-runafter",
		},
	}

	newItem := WorkItem{
		ID:               "new-cg-job-runafter",
		ConcurrencyGroup: "test-cg-runafter",
		CancelInProgress: true,
		RunAfter:         time.Now().Add(10 * time.Minute),
	}

	err := o.SubmitJob(ctx, newItem, logger)
	assert.NoError(t, err)
}

func TestOrchestrator_ProcessWorkItemInternal_RunAfterCancelError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Cancel", mock.Anything, mock.Anything).Return(assert.AnError)

	o := New(nil, mockSpawner, time.Second)
	ctx := context.Background()

	o.activeJobs["active-cg-runafter-err"] = JobInfo{
		ID:     "active-cg-runafter-err",
		Status: "active",
		WorkItem: WorkItem{
			ConcurrencyGroup: "test-cg-runafter-err",
		},
	}

	newItem := WorkItem{
		ID:               "new-cg-job-runafter-err",
		ConcurrencyGroup: "test-cg-runafter-err",
		CancelInProgress: true,
		RunAfter:         time.Now().Add(10 * time.Minute),
	}

	err := o.SubmitJob(ctx, newItem, logger)
	assert.NoError(t, err)
}

func TestOrchestrator_ProcessWorkItemInternal_MaxRetriesFallback(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockSpawner := new(MockSpawner)
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)

	o := New(nil, mockSpawner, time.Second)
	ctx := context.Background()

	newItem := WorkItem{
		ID:   "job-with-no-max-retries",
	}

	err := o.processWorkItemInternal(ctx, newItem, 0, false, logger)
	assert.NoError(t, err)
}
