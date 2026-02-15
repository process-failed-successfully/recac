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
	Observer     Observer
}

func New(poller Poller, spawner Spawner, pollInterval time.Duration) *Orchestrator {
	return &Orchestrator{
		Poller:       poller,
		Spawner:      spawner,
		PollInterval: pollInterval,
	}
}

// SetObserver sets the observer for the orchestrator
func (o *Orchestrator) SetObserver(obs Observer) {
	o.Observer = obs
}

// Run starts the orchestration loop
func (o *Orchestrator) Run(ctx context.Context, logger *slog.Logger) error {
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
			if o.Observer != nil {
				o.Observer.OnPollStart()
			}

			items, err := o.Poller.Poll(ctx, logger)
			if o.Observer != nil {
				o.Observer.OnPollEnd(len(items), err)
			}

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

					if o.Observer != nil {
						o.Observer.OnSpawnStart(item)
					}

					err := o.Spawner.Spawn(ctx, item)

					if o.Observer != nil {
						o.Observer.OnSpawnEnd(item, err)
					}

					if err != nil {
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
