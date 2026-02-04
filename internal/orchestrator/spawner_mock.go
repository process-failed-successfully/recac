package orchestrator

import (
	"context"
	"sync"
)

type MockSpawner struct {
	Spawned  []WorkItem
	SpawnErr error
	Mu       sync.Mutex
}

func NewMockSpawner() *MockSpawner {
	return &MockSpawner{}
}

func (m *MockSpawner) Spawn(ctx context.Context, item WorkItem) error {
	m.Mu.Lock()
	defer m.Mu.Unlock()
	m.Spawned = append(m.Spawned, item)
	return m.SpawnErr
}

func (m *MockSpawner) Cleanup(ctx context.Context, item WorkItem) error {
	return nil
}
