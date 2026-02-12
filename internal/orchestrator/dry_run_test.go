package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockPollerForDryRun implements Poller for testing DryRunPoller
type mockPollerForDryRun struct {
	items []WorkItem
}

func (m *mockPollerForDryRun) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	return m.items, nil
}

func (m *mockPollerForDryRun) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	// Should not be called by DryRunPoller
	return nil
}

func TestDryRunSpawner(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	spawner := NewDryRunSpawner(logger)

	item := WorkItem{ID: "TEST-1", Summary: "Test Task"}

	// Spawn should return nil
	err := spawner.Spawn(context.Background(), item)
	assert.NoError(t, err)

	// Cleanup should return nil
	err = spawner.Cleanup(context.Background(), item)
	assert.NoError(t, err)
}

func TestDryRunPoller(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	expectedItems := []WorkItem{{ID: "TEST-1"}}
	mock := &mockPollerForDryRun{items: expectedItems}

	poller := NewDryRunPoller(mock, logger)

	// Poll should return items from wrapped poller
	items, err := poller.Poll(context.Background(), logger)
	assert.NoError(t, err)
	assert.Equal(t, expectedItems, items)

	// UpdateStatus should NOT fail (and ideally just log)
	err = poller.UpdateStatus(context.Background(), items[0], "Done", "Fixed")
	assert.NoError(t, err)
}
