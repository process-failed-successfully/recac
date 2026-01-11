package orchestrator

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
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

// A silent logger for cleaner test output
var silentLogger = slog.New(slog.NewTextHandler(io.Discard, nil))
