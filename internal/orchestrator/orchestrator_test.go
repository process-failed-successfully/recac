package orchestrator

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"
)

type mockPoller struct {
	items []WorkItem
	err   error
}

func (m *mockPoller) Poll(ctx context.Context) ([]WorkItem, error) {
	return m.items, m.err
}

func (m *mockPoller) Claim(ctx context.Context, item WorkItem) error {
	return nil
}

func (m *mockPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	return nil
}

type mockSpawner struct {
	spawned []WorkItem
	err     error
}

func (m *mockSpawner) Spawn(ctx context.Context, item WorkItem) error {
	m.spawned = append(m.spawned, item)
	return m.err
}

func (m *mockSpawner) Cleanup(ctx context.Context, item WorkItem) error {
	return nil
}

func TestOrchestrator_Run(t *testing.T) {
	// Setup
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	poller := &mockPoller{
		items: []WorkItem{
			{ID: "TEST-1", Summary: "Task 1"},
			{ID: "TEST-2", Summary: "Task 2"},
		},
	}
	spawner := &mockSpawner{}

	orch := New(poller, spawner, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// rUN
	err := orch.Run(ctx, logger)

	// Assert
	if !errors.Is(err, context.DeadlineExceeded) && err != nil {
		t.Errorf("expected context deadline exceeded, got %v", err)
	}

	if len(spawner.spawned) < 2 {
		t.Errorf("expected at least 2 spawned items, got %d", len(spawner.spawned))
	}

	// Check content
	found1, found2 := false, false
	for _, item := range spawner.spawned {
		if item.ID == "TEST-1" {
			found1 = true
		}
		if item.ID == "TEST-2" {
			found2 = true
		}
	}

	if !found1 || !found2 {
		t.Error("did not spawn all items")
	}
}

func TestOrchestrator_Run_PollError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	poller := &mockPoller{
		err: errors.New("poll failed"),
	}
	spawner := &mockSpawner{}

	orch := New(poller, spawner, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := orch.Run(ctx, logger)

	if !errors.Is(err, context.DeadlineExceeded) && err != nil {
		t.Errorf("expected context deadline exceeded (cleanup), got %v", err)
	}

	if len(spawner.spawned) > 0 {
		t.Errorf("expected 0 spawned items, got %d", len(spawner.spawned))
	}
}
