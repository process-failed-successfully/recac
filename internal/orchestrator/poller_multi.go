package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
)

// MultiPoller aggregates multiple Poller instances.
type MultiPoller struct {
	pollers map[string]Poller
}

// NewMultiPoller creates a new MultiPoller with the given pollers.
func NewMultiPoller(pollers map[string]Poller) *MultiPoller {
	return &MultiPoller{
		pollers: pollers,
	}
}

func (mp *MultiPoller) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	var allItems []WorkItem
	for name, p := range mp.pollers {
		items, err := p.Poll(ctx, logger)
		if err != nil {
			logger.Error("Poller failed", "poller", name, "error", err)
			continue
		}
		if len(items) > 0 {
			allItems = append(allItems, items...)
		}
	}

	return allItems, nil
}

func (mp *MultiPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	poller, ok := mp.pollers[item.Source]
	if !ok {
		return fmt.Errorf("unknown source for work item: %s", item.Source)
	}

	return poller.UpdateStatus(ctx, item, status, comment)
}
