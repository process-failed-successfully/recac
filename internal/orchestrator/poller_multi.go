package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// MultiPoller aggregates multiple Poller instances.
type MultiPoller struct {
	pollers map[string]Poller
	mu      sync.RWMutex
}

// NewMultiPoller creates a new MultiPoller.
func NewMultiPoller() *MultiPoller {
	return &MultiPoller{
		pollers: make(map[string]Poller),
	}
}

// AddPoller adds a poller with a unique name.
func (mp *MultiPoller) AddPoller(name string, p Poller) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.pollers[name] = p
}

// Poll aggregates work items from all registered pollers.
func (mp *MultiPoller) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	var allItems []WorkItem
	var errs []error

	// Poll sequentially for now to avoid race conditions or log interleaving complexity
	for name, p := range mp.pollers {
		items, err := p.Poll(ctx, logger.With("poller", name))
		if err != nil {
			logger.Error("Poller failed", "poller", name, "error", err)
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			continue
		}

		for _, item := range items {
			// Tag item with source so we know where to route updates back
			item.Source = name
			allItems = append(allItems, item)
		}
	}

	if len(allItems) == 0 && len(errs) > 0 {
		// If all pollers failed, return an error.
		// If some succeeded and some failed, we prefer to return the items we found.
		if len(errs) == len(mp.pollers) {
			return nil, fmt.Errorf("all pollers failed: %v", errs)
		}
	}

	return allItems, nil
}

// UpdateStatus routes the update to the correct poller.
func (mp *MultiPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	mp.mu.RLock()
	p, ok := mp.pollers[item.Source]
	mp.mu.RUnlock()

	if !ok {
		return fmt.Errorf("poller source '%s' not found for item %s", item.Source, item.ID)
	}

	return p.UpdateStatus(ctx, item, status, comment)
}
