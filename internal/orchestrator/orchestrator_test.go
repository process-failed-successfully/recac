package orchestrator

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

// --- Mocks ---

type mockPoller struct {
	pollItems []WorkItem
	pollErr   error
	claimErr  error
	updateErr error

	// Track calls
	mu           sync.Mutex
	polled       bool
	claimedItems []WorkItem
	updatedItems map[string]string // map[ID]status
	claimCb      func(item WorkItem) error
}

func (m *mockPoller) Poll(ctx context.Context) ([]WorkItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.polled {
		// Return no items on subsequent polls to prevent reprocessing.
		return []WorkItem{}, nil
	}
	m.polled = true
	return m.pollItems, m.pollErr
}

func (m *mockPoller) Claim(ctx context.Context, item WorkItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.claimCb != nil {
		return m.claimCb(item)
	}

	if m.claimErr != nil {
		return m.claimErr
	}
	m.claimedItems = append(m.claimedItems, item)
	return nil
}

func (m *mockPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.updatedItems == nil {
		m.updatedItems = make(map[string]string)
	}
	m.updatedItems[item.ID] = status
	return m.updateErr
}

type mockSpawner struct {
	spawnErr error

	mu      sync.Mutex
	spawned []WorkItem
}

func (m *mockSpawner) Spawn(ctx context.Context, item WorkItem) error {
	m.mu.Lock()
	m.spawned = append(m.spawned, item)
	m.mu.Unlock()
	return m.spawnErr
}

func (m *mockSpawner) Cleanup(ctx context.Context, item WorkItem) error {
	return nil
}

// --- Tests ---

func TestOrchestrator_Run(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	poller := &mockPoller{
		pollItems: []WorkItem{
			{ID: "TEST-1", Summary: "Task 1"},
			{ID: "TEST-2", Summary: "Task 2"},
		},
	}
	spawner := &mockSpawner{}

	orch := New(poller, spawner, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := orch.Run(ctx, logger)

	if !errors.Is(err, context.DeadlineExceeded) && err != nil {
		t.Errorf("expected context deadline exceeded, got %v", err)
	}

	spawner.mu.Lock()
	defer spawner.mu.Unlock()
	if len(spawner.spawned) < 2 {
		t.Errorf("expected at least 2 spawned items, got %d", len(spawner.spawned))
	}

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
		pollErr: errors.New("poll failed"),
	}
	spawner := &mockSpawner{}

	orch := New(poller, spawner, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := orch.Run(ctx, logger)

	if !errors.Is(err, context.DeadlineExceeded) && err != nil {
		t.Errorf("expected context deadline exceeded, got %v", err)
	}

	spawner.mu.Lock()
	defer spawner.mu.Unlock()
	if len(spawner.spawned) > 0 {
		t.Errorf("expected 0 spawned items, got %d", len(spawner.spawned))
	}
}

func TestOrchestrator_Run_NoWorkItems(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	poller := &mockPoller{
		pollItems: []WorkItem{},
	}
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 10*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := orch.Run(ctx, logger)

	if !errors.Is(err, context.DeadlineExceeded) && err != nil {
		t.Errorf("expected context deadline exceeded, got %v", err)
	}
	spawner.mu.Lock()
	defer spawner.mu.Unlock()
	if len(spawner.spawned) != 0 {
		t.Errorf("expected 0 spawned items, got %d", len(spawner.spawned))
	}
}

func TestOrchestrator_Run_ClaimError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	poller := &mockPoller{
		pollItems: []WorkItem{{ID: "FAIL-CLAIM"}, {ID: "OK-CLAIM"}},
		claimCb: func(item WorkItem) error {
			if item.ID == "FAIL-CLAIM" {
				return errors.New("claim failed")
			}
			return nil
		},
	}
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 10*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_ = orch.Run(ctx, logger)

	spawner.mu.Lock()
	defer spawner.mu.Unlock()

	if len(spawner.spawned) != 1 {
		t.Fatalf("expected 1 spawned item, got %d", len(spawner.spawned))
	}
	if spawner.spawned[0].ID != "OK-CLAIM" {
		t.Errorf("expected item 'OK-CLAIM' to be spawned, but got '%s'", spawner.spawned[0].ID)
	}
}

func TestOrchestrator_Run_SpawnError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	item := WorkItem{ID: "TEST-1"}
	poller := &mockPoller{
		pollItems: []WorkItem{item},
	}
	spawner := &mockSpawner{
		spawnErr: errors.New("spawn failed"),
	}

	orch := New(poller, spawner, 10*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_ = orch.Run(ctx, logger)

	spawner.mu.Lock()
	if len(spawner.spawned) != 1 {
		t.Fatalf("expected 1 spawn attempt, got %d", len(spawner.spawned))
	}
	if spawner.spawned[0].ID != "TEST-1" {
		t.Errorf("expected spawned item with ID TEST-1, got %s", spawner.spawned[0].ID)
	}
	spawner.mu.Unlock()

	poller.mu.Lock()
	defer poller.mu.Unlock()
	if status, ok := poller.updatedItems["TEST-1"]; !ok || status != "Failed" {
		t.Errorf("expected item status to be updated to 'Failed', but got '%v'", poller.updatedItems)
	}
}
