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
	RunOnce      bool
}

func New(poller Poller, spawner Spawner, pollInterval time.Duration) *Orchestrator {
	return &Orchestrator{
		Poller:       poller,
		Spawner:      spawner,
		PollInterval: pollInterval,
	}
}

// Run starts the orchestration loop
func (o *Orchestrator) Run(ctx context.Context, logger *slog.Logger) error {
	logger.Info("Starting Orchestrator", "interval", o.PollInterval, "run_once", o.RunOnce)

	// Use a WaitGroup to track running spawns/jobs if we want graceful shutdown
	var wg sync.WaitGroup

	// Helper function for the polling logic
	pollAndSpawn := func() {
		logger.Debug("Polling for work...")
		items, err := o.Poller.Poll(ctx, logger)
		if err != nil {
			logger.Error("Failed to poll for work", "error", err)
			return
		}

		if len(items) == 0 {
			return
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

	// First run immediately
	pollAndSpawn()

	if o.RunOnce {
		logger.Info("RunOnce enabled, waiting for jobs and shutting down...")
		wg.Wait()
		return nil
	}

	ticker := time.NewTicker(o.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Orchestrator shutting down...")
			wg.Wait()
			return ctx.Err()
		case <-ticker.C:
			pollAndSpawn()
		}
	}
}
