package orchestrator

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockAutoHealSpawner struct {
	spawnCount int
	lastItem   WorkItem
}

func (m *mockAutoHealSpawner) Spawn(ctx context.Context, item WorkItem) error {
	m.spawnCount++
	m.lastItem = item
	if m.spawnCount == 1 {
		return fmt.Errorf("simulated spawn failure on first attempt")
	}
	return nil
}

func (m *mockAutoHealSpawner) Cleanup(ctx context.Context, item WorkItem) error { return nil }
func (m *mockAutoHealSpawner) Cancel(ctx context.Context, jobID string) error   { return nil }
func (m *mockAutoHealSpawner) GetLogs(ctx context.Context, jobID string) (io.ReadCloser, error) {
	// Return some fake logs to simulate a container that failed and has logs
	logs := "Fake log line 1\nFake log line 2\nError occurred"
	return io.NopCloser(strings.NewReader(logs)), nil
}
func (m *mockAutoHealSpawner) Ping(ctx context.Context) error { return nil }

func TestOrchestrator_AutoHeal(t *testing.T) {
	poller := &mockPoller{}
	spawner := &mockAutoHealSpawner{}
	orch := New(poller, spawner, 1*time.Second)
	orch.MaxRetries = 2
	orch.RetryDelay = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Run orchestrator
	go orch.Run(ctx, logger)

	item := WorkItem{
		ID:          "AUTOHEAL-1",
		Summary:     "Auto-Heal Test",
		Description: "Initial description",
		AutoHeal:    true,
	}

	err := orch.SubmitJob(ctx, item, logger)
	assert.NoError(t, err)

	// Wait for the job to eventually complete (after 1 retry)
	assert.Eventually(t, func() bool {
		job, err := orch.GetJob("AUTOHEAL-1")
		if err != nil {
			return false
		}
		return job.Status == "Completed"
	}, 2*time.Second, 10*time.Millisecond)

	// Verify spawner calls and descriptions
	assert.Equal(t, 2, spawner.spawnCount, "spawner should have been called 2 times")

	// The second call (lastItem) should have the updated description
	expectedLogFragment := "Fake log line 1"
	expectedErrorFragment := "simulated spawn failure on first attempt"

	assert.Contains(t, spawner.lastItem.Description, "Initial description", "Should contain original description")
	assert.Contains(t, spawner.lastItem.Description, "Auto-Heal Attempt 1", "Should contain auto-heal header")
	assert.Contains(t, spawner.lastItem.Description, expectedErrorFragment, "Should contain error message")
	assert.Contains(t, spawner.lastItem.Description, expectedLogFragment, "Should contain fetched logs")
}
