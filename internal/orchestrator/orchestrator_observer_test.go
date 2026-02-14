package orchestrator

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type MockObserver struct {
	PollStartCount int
	PollEndCount   int
	SpawnStartCount int
	SpawnEndCount   int
	mu sync.Mutex
}

func (m *MockObserver) OnPollStart() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PollStartCount++
}

func (m *MockObserver) OnPollEnd(count int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PollEndCount++
}

func (m *MockObserver) OnSpawnStart(item WorkItem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SpawnStartCount++
}

func (m *MockObserver) OnSpawnEnd(item WorkItem, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SpawnEndCount++
}

type ObserverMockPoller struct {
	Items []WorkItem
	Err   error
}

func (m *ObserverMockPoller) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	return m.Items, m.Err
}

func (m *ObserverMockPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	return nil
}

type ObserverMockSpawner struct{}

func (m *ObserverMockSpawner) Spawn(ctx context.Context, item WorkItem) error {
	return nil
}

func (m *ObserverMockSpawner) Cleanup(ctx context.Context, item WorkItem) error {
	return nil
}

func TestOrchestrator_Observer(t *testing.T) {
	// Setup
	poller := &ObserverMockPoller{
		Items: []WorkItem{{ID: "TEST-1"}},
	}
	spawner := &ObserverMockSpawner{}
	observer := &MockObserver{}

	// Interval must be short enough to trigger, but long enough to control
	// We only need 1 cycle.
	orch := New(poller, spawner, 10*time.Millisecond)
	orch.SetObserver(observer)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	logger := slog.Default()

	// Run
	go func() {
		_ = orch.Run(ctx, logger)
	}()

	// Wait for context timeout
	<-ctx.Done()

	// Check observer
	observer.mu.Lock()
	defer observer.mu.Unlock()

	assert.GreaterOrEqual(t, observer.PollStartCount, 1, "Should have started polling at least once")
	assert.GreaterOrEqual(t, observer.PollEndCount, 1, "Should have ended polling at least once")
	assert.GreaterOrEqual(t, observer.SpawnStartCount, 1, "Should have started spawning at least once")
	assert.GreaterOrEqual(t, observer.SpawnEndCount, 1, "Should have ended spawning at least once")
}
