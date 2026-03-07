package orchestrator

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// A thread-safe mock poller that simulates claiming items.
type mockPoller struct {
	items          map[string]WorkItem
	itemsMu        sync.Mutex
	pollErr        error
	pingErr        error
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

func (m *mockPoller) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	m.itemsMu.Lock()
	defer m.itemsMu.Unlock()
	if m.pollErr != nil {
		return nil, m.pollErr
	}
	var result []WorkItem
	for _, item := range m.items {
		result = append(result, item)
	}
	// Clear items after polling to simulate them being claimed
	m.items = make(map[string]WorkItem)
	return result, nil
}

func (m *mockPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	m.updateStatusMu.Lock()
	defer m.updateStatusMu.Unlock()
	m.updateStatus[item.ID] = status
	return nil
}

func (m *mockPoller) Ping(ctx context.Context) error {
	return m.pingErr
}

type mockSpawner struct {
	spawned  []WorkItem
	spawnErr error
	pingErr  error
	mu       sync.Mutex
	blockCh  chan struct{}
}

func (m *mockSpawner) Spawn(ctx context.Context, item WorkItem) error {
	if m.blockCh != nil {
		<-m.blockCh
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.spawned = append(m.spawned, item)
	return m.spawnErr
}

func (m *mockSpawner) Cleanup(ctx context.Context, item WorkItem) error {
	return nil
}

func (m *mockSpawner) Cancel(ctx context.Context, jobID string) error {
	return nil
}

func (m *mockSpawner) Ping(ctx context.Context) error {
	return m.pingErr
}

func (m *mockSpawner) GetLogs(ctx context.Context, jobID string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("mock logs")), nil
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
	polledItems, _ := poller.Poll(context.Background(), silentLogger)
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
				items, _ := p.Poll(context.Background(), silentLogger)
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

func TestOrchestrator_SubmitJob(t *testing.T) {
	poller := newMockPoller(nil)
	blockCh := make(chan struct{})
	spawner := &mockSpawner{blockCh: blockCh}
	orch := New(poller, spawner, 50*time.Millisecond)

	ctx := context.Background()

	// 1. Submit a valid job
	item := WorkItem{ID: "MANUAL-1", Summary: "Manual Job"}
	err := orch.SubmitJob(ctx, item, silentLogger)
	require.NoError(t, err)

	// Check that it's in active jobs
	orch.mu.Lock()
	_, exists := orch.activeJobs["MANUAL-1"]
	orch.mu.Unlock()
	assert.True(t, exists, "Job should be in activeJobs")

	// 2. Submit duplicate job immediately (it should be active because spawn is blocked)
	err = orch.SubmitJob(ctx, item, silentLogger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already active")

	// Unblock spawner
	close(blockCh)

	// Wait for spawner goroutine to finish
	time.Sleep(50 * time.Millisecond)

	spawner.mu.Lock()
	assert.Len(t, spawner.spawned, 1)
	assert.Equal(t, "MANUAL-1", spawner.spawned[0].ID)
	spawner.mu.Unlock()

	// 3. Submit another job (should work now that we unblocked, though blockCh is closed so it won't block)
	item2 := WorkItem{ID: "MANUAL-2", Summary: "Manual Job 2"}
	err = orch.SubmitJob(ctx, item2, silentLogger)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	spawner.mu.Lock()
	assert.Len(t, spawner.spawned, 2)
	spawner.mu.Unlock()
}

func TestOrchestrator_ClearPendingJobs(t *testing.T) {
	poller := newMockPoller(nil)
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 50*time.Millisecond)

	ctx := context.Background()

	// Add a pending job that depends on a non-existent job
	err := orch.SubmitJob(ctx, WorkItem{ID: "JOB-PENDING", DependsOn: []string{"JOB-UNKNOWN"}}, silentLogger)
	require.NoError(t, err)

	// Verify it is pending
	orch.mu.Lock()
	_, isPending := orch.pendingJobs["JOB-PENDING"]
	orch.mu.Unlock()
	assert.True(t, isPending, "Job should be pending")

	// Clear pending jobs
	count := orch.ClearPendingJobs(ctx, silentLogger)
	assert.Equal(t, 1, count)

	// Verify pending jobs is empty
	orch.mu.Lock()
	pendingCount := len(orch.pendingJobs)
	orch.mu.Unlock()
	assert.Equal(t, 0, pendingCount)

	// Verify the job was recorded as canceled in history
	completed := orch.GetCompletedJobs()
	assert.Len(t, completed, 1)
	assert.Equal(t, "JOB-PENDING", completed[0].ID)
	assert.Equal(t, "Canceled", completed[0].Status)
	assert.Equal(t, "Canceled from pending queue", completed[0].Error)
}

func TestOrchestrator_CancelAllJobs(t *testing.T) {
	poller := newMockPoller(nil)
	blockCh := make(chan struct{})
	spawner := &mockSpawner{blockCh: blockCh}
	orch := New(poller, spawner, 50*time.Millisecond)

	ctx := context.Background()

	// Submit multiple jobs that will block in spawn
	err := orch.SubmitJob(ctx, WorkItem{ID: "JOB-1"}, silentLogger)
	require.NoError(t, err)

	err = orch.SubmitJob(ctx, WorkItem{ID: "JOB-2"}, silentLogger)
	require.NoError(t, err)

	// Verify they are active
	activeJobs := orch.GetActiveJobs()
	assert.Len(t, activeJobs, 2)

	// Cancel all jobs
	count, err := orch.CancelAllJobs(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Unblock spawner to let them finish
	close(blockCh)

	// Wait for goroutines to finish
	time.Sleep(50 * time.Millisecond)

	// Verify active jobs is now 0
	activeJobs = orch.GetActiveJobs()
	assert.Len(t, activeJobs, 0)
}

func TestOrchestrator_ForcePoll(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockSpawner := new(MockSpawner)

	// Single item to be returned by Poll
	item := WorkItem{ID: "TASK-1"}
	mockPoller := newMockPoller([]WorkItem{item})
	mockSpawner.On("Spawn", mock.Anything, mock.Anything).Return(nil)

	// Set a very long interval so it won't tick normally during the test.
	orch := New(mockPoller, mockSpawner, 1*time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// WaitGroup to track completion
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		_ = orch.Run(ctx, logger)
	}()

	// Wait for the initial poll (it happens before the loop).
	time.Sleep(50 * time.Millisecond)

	// Now add a new item and trigger a force poll
	newItem := WorkItem{ID: "TASK-2"}
	mockPoller.itemsMu.Lock()
	mockPoller.items["TASK-2"] = newItem
	mockPoller.itemsMu.Unlock()

	orch.ForcePoll()

	// Wait a moment for the poll to process
	time.Sleep(50 * time.Millisecond)

	// Check if TASK-2 was spawned
	mockSpawner.AssertCalled(t, "Spawn", mock.Anything, mock.MatchedBy(func(i WorkItem) bool {
		return i.ID == "TASK-2"
	}))

	// Cleanup
	cancel()
	wg.Wait()
}
