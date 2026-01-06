package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// FilePoller reads work items from a local JSON file
type FilePoller struct {
	path      string
	processed map[string]bool
	mu        sync.Mutex
}

func NewFilePoller(path string) *FilePoller {
	return &FilePoller{
		path:      path,
		processed: make(map[string]bool),
	}
}

func (p *FilePoller) Poll(ctx context.Context) ([]WorkItem, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, err := os.Stat(p.path); os.IsNotExist(err) {
		return nil, nil // No work file found yet
	}

	data, err := os.ReadFile(p.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read work file: %w", err)
	}

	var items []WorkItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("failed to unmarshal work items: %w", err)
	}

	// Filter out already claimed items
	var newItems []WorkItem
	for _, item := range items {
		if !p.processed[item.ID] {
			newItems = append(newItems, item)
		}
	}

	return newItems, nil
}

func (p *FilePoller) Claim(ctx context.Context, item WorkItem) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.processed[item.ID] = true
	return nil
}

func (p *FilePoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	// No-op for file poller usually, but could log
	fmt.Printf("[FilePoller] Item %s status updated to %s: %s\n", item.ID, status, comment)
	return nil
}
