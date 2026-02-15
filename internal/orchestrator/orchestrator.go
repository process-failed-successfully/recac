package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type Orchestrator struct {
	Poller       Poller
	Spawner      Spawner
	PollInterval time.Duration
	DryRun       bool
}

func New(poller Poller, spawner Spawner, pollInterval time.Duration) *Orchestrator {
	return &Orchestrator{
		Poller:       poller,
		Spawner:      spawner,
		PollInterval: pollInterval,
	}
}

// SetDryRun enables or disables dry-run mode
func (o *Orchestrator) SetDryRun(dryRun bool) {
	o.DryRun = dryRun
}

// Run starts the orchestration loop
func (o *Orchestrator) Run(ctx context.Context, logger *slog.Logger) error {
	// Dry Run Mode
	if o.DryRun {
		logger.Info("Running in DRY-RUN mode")
		fmt.Println("--- DRY RUN: Polling for work items ---")

		items, err := o.Poller.Poll(ctx, logger)
		if err != nil {
			logger.Error("Failed to poll for work", "error", err)
			return err
		}

		if len(items) == 0 {
			logger.Info("No work items found")
			fmt.Println("No work items found.")
			return nil
		}

		logger.Info("Found work items", "count", len(items))
		fmt.Printf("Found %d work items:\n", len(items))

		for _, item := range items {
			fmt.Printf("- [%s] %s\n  Repo: %s\n", item.ID, item.Summary, item.RepoURL)
		}
		fmt.Println("--- End of Dry Run ---")
		return nil
	}

	logger.Info("Starting Orchestrator", "interval", o.PollInterval)
	ticker := time.NewTicker(o.PollInterval)
	defer ticker.Stop()

	// Use a WaitGroup to track running spawns/jobs if we want graceful shutdown
	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			logger.Info("Orchestrator shutting down...")
			wg.Wait()
			return ctx.Err()
		case <-ticker.C:
			// Poll for work
			logger.Debug("Polling for work...")
			items, err := o.Poller.Poll(ctx, logger)
			if err != nil {
				logger.Error("Failed to poll for work", "error", err)
				continue
			}

			if len(items) == 0 {
				continue
			}

			logger.Info("Found work items", "count", len(items))

			for _, item := range items {
				wg.Add(1)
				go func(item WorkItem) {
					defer wg.Done()
					logger.Info("Spawning agent for item", "id", item.ID)

					if err := o.Spawner.Spawn(ctx, item); err != nil {
						logger.Error("Failed to spawn agent", "id", item.ID, "error", err)
						// Update status to Failed
						_ = o.Poller.UpdateStatus(ctx, item, "Failed", fmt.Sprintf("Failed to spawn agent: %v", err))
					} else {
						// Success? K8s Jobs are fire-and-forget from Spawner perspective usually,
						// but status updates might happen asynchronously.
						// For now, Spawn() implies "Started".
						logger.Info("Agent spawned successfully", "id", item.ID)
					}
				}(item)
			}
		}
	}
}
