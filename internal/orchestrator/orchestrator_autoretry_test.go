package orchestrator

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockRetrySpawner struct {
	spawnCount int
	failCount  int
}

func (m *mockRetrySpawner) Spawn(ctx context.Context, item WorkItem) error {
	m.spawnCount++
	if m.spawnCount <= m.failCount {
		return fmt.Errorf("simulated failure %d", m.spawnCount)
	}
	return nil
}

func (m *mockRetrySpawner) Cleanup(ctx context.Context, item WorkItem) error { return nil }
func (m *mockRetrySpawner) Cancel(ctx context.Context, jobID string) error  { return nil }
func (m *mockRetrySpawner) GetLogs(ctx context.Context, jobID string) (io.ReadCloser, error) {
	return nil, nil
}
func (m *mockRetrySpawner) Ping(ctx context.Context) error { return nil }

func TestOrchestrator_AutoRetry_Success(t *testing.T) {
	poller := &mockPoller{}
	spawner := &mockRetrySpawner{failCount: 2} // Fails twice, succeeds third time
	orch := New(poller, spawner, 1*time.Second)
	orch.MaxRetries = 3
	orch.RetryDelay = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Run orchestrator
	go orch.Run(ctx, logger)

	item := WorkItem{ID: "RETRY-1", Summary: "Retry Test"}
	err := orch.SubmitJob(ctx, item, logger)
	assert.NoError(t, err)

	// Wait for the job to eventually complete (after 2 retries)
	assert.Eventually(t, func() bool {
		job, err := orch.GetJob("RETRY-1")
		if err != nil {
			return false
		}
		return job.Status == "Completed"
	}, 2*time.Second, 10*time.Millisecond)

	// Verify retry count
	job, err := orch.GetJob("RETRY-1")
	assert.NoError(t, err)
	assert.Equal(t, 2, job.RetryCount, "job should have retried exactly 2 times")
	assert.Equal(t, 3, spawner.spawnCount, "spawner should have been called 3 times total")
}

func TestOrchestrator_AutoRetry_Exhausted(t *testing.T) {
	poller := &mockPoller{}
	spawner := &mockRetrySpawner{failCount: 5} // Fails 5 times
	orch := New(poller, spawner, 1*time.Second)
	orch.MaxRetries = 2
	orch.RetryDelay = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Run orchestrator
	go orch.Run(ctx, logger)

	item := WorkItem{ID: "RETRY-2", Summary: "Retry Exhaust Test"}
	err := orch.SubmitJob(ctx, item, logger)
	assert.NoError(t, err)

	// Wait for the job to eventually fail (after exhausting retries)
	assert.Eventually(t, func() bool {
		job, err := orch.GetJob("RETRY-2")
		if err != nil {
			return false
		}
		return job.Status == "Failed"
	}, 2*time.Second, 10*time.Millisecond)

	// Verify retry count
	job, err := orch.GetJob("RETRY-2")
	assert.NoError(t, err)
	assert.Equal(t, 2, job.RetryCount, "job should have retried exactly 2 times before failing")
	assert.Equal(t, 3, spawner.spawnCount, "spawner should have been called 3 times total (initial + 2 retries)")
}

func TestOrchestrator_AutoRetry_CancelDuringRetry(t *testing.T) {
	poller := &mockPoller{}
	spawner := &mockRetrySpawner{failCount: 5}
	orch := New(poller, spawner, 1*time.Second)
	orch.MaxRetries = 3
	orch.RetryDelay = 500 * time.Millisecond // Long enough to catch it in pendingJobs

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Do not run orchestrator loop here to perfectly control timing,
	// or we can run it and just catch it.
	// Actually we submit, wait for it to be in "Retrying" state, then cancel.
	err := orch.processWorkItem(ctx, WorkItem{ID: "RETRY-CANCEL", Summary: "Cancel Test"}, 0, logger)
	assert.NoError(t, err)

	// Wait for first failure and move to Retrying
	assert.Eventually(t, func() bool {
		job, err := orch.GetJob("RETRY-CANCEL")
		if err != nil {
			return false
		}
		return job.Status == "Retrying"
	}, 1*time.Second, 10*time.Millisecond)

	// Cancel it while it's waiting
	err = orch.CancelJob(ctx, "RETRY-CANCEL")
	assert.NoError(t, err)

	// Verify it's Canceled
	job, err := orch.GetJob("RETRY-CANCEL")
	assert.NoError(t, err)
	assert.Equal(t, "Canceled", job.Status)
}
