package orchestrator

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A thread-safe mock poller that simulates claiming items.
type mockPoller struct {
	items          map[string]WorkItem
	itemsMu        sync.Mutex
	pollErr        error
	claimErr       error
	updateStatus   map[string]string
	updateStatusMu sync.Mutex
}

func newMockPoller(items []WorkItem) *mockPoller {
	itemMap := make(map[string]WorkItem)
	for _, item := range items {
		itemMap[item.ID] = item
	}
	return &mockPoller{
		items:        itemMap,
		updateStatus: make(map[string]string),
	}
}

func (m *mockPoller) Poll(ctx context.Context) ([]WorkItem, error) {
	m.itemsMu.Lock()
	defer m.itemsMu.Unlock()
	if m.pollErr != nil {
		return nil, m.pollErr
	}
	var result []WorkItem
	for _, item := range m.items {
		result = append(result, item)
	}
	return result, nil
}

func (m *mockPoller) Claim(ctx context.Context, item WorkItem) error {
	if m.claimErr != nil {
		return m.claimErr
	}
	m.itemsMu.Lock()
	defer m.itemsMu.Unlock()
	if _, ok := m.items[item.ID]; !ok {
		return errors.New("item not found")
	}
	delete(m.items, item.ID)
	return nil
}

func (m *mockPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	m.updateStatusMu.Lock()
	defer m.updateStatusMu.Unlock()
	m.updateStatus[item.ID] = status
	return nil
}

type mockSpawner struct {
	spawned  []WorkItem
	spawnErr error
	mu       sync.Mutex
}

func (m *mockSpawner) Spawn(ctx context.Context, item WorkItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.spawned = append(m.spawned, item)
	return m.spawnErr
}

func (m *mockSpawner) Cleanup(ctx context.Context, item WorkItem) error {
	return nil
}

// A silent logger for cleaner test output
var silentLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestOrchestrator_Run_Success(t *testing.T) {
	poller := newMockPoller([]WorkItem{
		{ID: "TEST-1", Summary: "Task 1"},
		{ID: "TEST-2", Summary: "Task 2"},
	})
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := orch.Run(ctx, silentLogger)

	require.ErrorIs(t, err, context.DeadlineExceeded)

	spawner.mu.Lock()
	defer spawner.mu.Unlock()

	// Check that both items were spawned exactly once.
	assert.Len(t, spawner.spawned, 2)
	found := make(map[string]bool)
	for _, item := range spawner.spawned {
		found[item.ID] = true
	}
	assert.True(t, found["TEST-1"])
	assert.True(t, found["TEST-2"])

	// Check that poller has no more items
	polledItems, _ := poller.Poll(context.Background())
	assert.Empty(t, polledItems)
}

func TestOrchestrator_Run_PollError(t *testing.T) {
	poller := newMockPoller(nil)
	poller.pollErr = errors.New("poll failed")
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 10*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := orch.Run(ctx, silentLogger)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Empty(t, spawner.spawned)
}

func TestOrchestrator_Run_Scenarios(t *testing.T) {
	testCases := []struct {
		name               string
		setupPoller        func() *mockPoller
		setupSpawner       func() *mockSpawner
		timeout            time.Duration
		expectedSpawnCount int
		expectedStatus     map[string]string
		verifyPoller       func(t *testing.T, p *mockPoller)
	}{
		{
			name: "Claim Error",
			setupPoller: func() *mockPoller {
				p := newMockPoller([]WorkItem{{ID: "TEST-1"}})
				p.claimErr = errors.New("claim failed")
				return p
			},
			setupSpawner:       func() *mockSpawner { return &mockSpawner{} },
			timeout:            50 * time.Millisecond,
			expectedSpawnCount: 0,
			expectedStatus:     map[string]string{},
			verifyPoller: func(t *testing.T, p *mockPoller) {
				// Item should NOT have been claimed/removed
				items, _ := p.Poll(context.Background())
				assert.Len(t, items, 1)
			},
		},
		{
			name:        "Spawn Error",
			setupPoller: func() *mockPoller { return newMockPoller([]WorkItem{{ID: "TEST-1"}}) },
			setupSpawner: func() *mockSpawner {
				return &mockSpawner{spawnErr: errors.New("spawn failed")}
			},
			timeout:            50 * time.Millisecond,
			expectedSpawnCount: 1,
			expectedStatus:     map[string]string{"TEST-1": "Failed"},
			verifyPoller: func(t *testing.T, p *mockPoller) {
				// Item should have been claimed/removed
				items, _ := p.Poll(context.Background())
				assert.Empty(t, items)
			},
		},
		{
			name:               "No Work",
			setupPoller:        func() *mockPoller { return newMockPoller(nil) },
			setupSpawner:       func() *mockSpawner { return &mockSpawner{} },
			timeout:            50 * time.Millisecond,
			expectedSpawnCount: 0,
			expectedStatus:     map[string]string{},
			verifyPoller:       nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			poller := tc.setupPoller()
			spawner := tc.setupSpawner()
			orch := New(poller, spawner, 10*time.Millisecond)
			ctx, cancel := context.WithTimeout(context.Background(), tc.timeout)
			defer cancel()

			err := orch.Run(ctx, silentLogger)

			require.ErrorIs(t, err, context.DeadlineExceeded)

			spawner.mu.Lock()
			assert.Len(t, spawner.spawned, tc.expectedSpawnCount)
			spawner.mu.Unlock()

			poller.updateStatusMu.Lock()
			assert.Equal(t, tc.expectedStatus, poller.updateStatus)
			poller.updateStatusMu.Unlock()

			if tc.verifyPoller != nil {
				tc.verifyPoller(t, poller)
			}
		})
	}
}

func TestOrchestrator_Run_GracefulShutdown(t *testing.T) {
	poller := newMockPoller([]WorkItem{{ID: "TEST-1"}})
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := orch.Run(ctx, silentLogger)
		assert.ErrorIs(t, err, context.Canceled)
	}()

	// Allow orchestrator to start and poll once
	time.Sleep(100 * time.Millisecond)

	// Verify it was spawned
	spawner.mu.Lock()
	require.Len(t, spawner.spawned, 1)
	assert.Equal(t, "TEST-1", spawner.spawned[0].ID)
	spawner.mu.Unlock()

	// Now cancel and wait for shutdown
	cancel()
	wg.Wait()
}
