package orchestrator

import (
	"context"
	"log/slog"
)

// DryRunSpawner implements the Spawner interface but only logs actions.
type DryRunSpawner struct {
	Logger *slog.Logger
}

func NewDryRunSpawner(logger *slog.Logger) *DryRunSpawner {
	return &DryRunSpawner{Logger: logger}
}

func (s *DryRunSpawner) Spawn(ctx context.Context, item WorkItem) error {
	s.Logger.Info("DRY RUN: Would spawn agent",
		"item_id", item.ID,
		"summary", item.Summary,
		"repo_url", item.RepoURL,
	)
	return nil
}

func (s *DryRunSpawner) Cleanup(ctx context.Context, item WorkItem) error {
	s.Logger.Info("DRY RUN: Would cleanup agent", "item_id", item.ID)
	return nil
}

// DryRunPoller implements the Poller interface, wrapping another Poller.
// It performs real polling (to find work) but prevents status updates.
type DryRunPoller struct {
	Wrapped Poller
	Logger  *slog.Logger
}

func NewDryRunPoller(wrapped Poller, logger *slog.Logger) *DryRunPoller {
	return &DryRunPoller{
		Wrapped: wrapped,
		Logger:  logger,
	}
}

func (p *DryRunPoller) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	// Delegate to the real poller to find actual work
	return p.Wrapped.Poll(ctx, logger)
}

func (p *DryRunPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	p.Logger.Info("DRY RUN: Would update status",
		"item_id", item.ID,
		"status", status,
		"comment", comment,
	)
	return nil
}
