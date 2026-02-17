package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

// Janitor periodically cleans up old agent containers.
type Janitor struct {
	Client   DockerClient
	Logger   *slog.Logger
	Interval time.Duration
	MaxAge   time.Duration
	DryRun   bool
}

// NewJanitor creates a new Janitor.
func NewJanitor(logger *slog.Logger, client DockerClient, interval, maxAge time.Duration, dryRun bool) *Janitor {
	return &Janitor{
		Client:   client,
		Logger:   logger,
		Interval: interval,
		MaxAge:   maxAge,
		DryRun:   dryRun,
	}
}

// Start starts the janitor loop in a background goroutine.
func (j *Janitor) Start(ctx context.Context) {
	go func() {
		j.Logger.Info("Janitor started", "interval", j.Interval, "max_age", j.MaxAge, "dry_run", j.DryRun)
		ticker := time.NewTicker(j.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				j.Logger.Info("Janitor stopping")
				return
			case <-ticker.C:
				if err := j.Cleanup(ctx); err != nil {
					j.Logger.Error("Janitor cleanup failed", "error", err)
				}
			}
		}
	}()
}

// Cleanup performs a single cleanup cycle.
func (j *Janitor) Cleanup(ctx context.Context) error {
	j.Logger.Debug("Janitor running cleanup")

	// Filter for containers created by recac-orchestrator
	opts := container.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", "created-by=recac-orchestrator"),
		),
	}

	containers, err := j.Client.ListContainers(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	removedCount := 0
	for _, c := range containers {
		// Check age
		// Use Created field (Unix timestamp) if available
		createdAt := time.Unix(c.Created, 0)
		age := time.Since(createdAt)

		if age > j.MaxAge {
			if j.DryRun {
				j.Logger.Info("Would remove container", "id", c.ID[:12], "age", age, "names", c.Names)
			} else {
				j.Logger.Info("Removing container", "id", c.ID[:12], "age", age)
				if err := j.Client.RemoveContainer(ctx, c.ID, true); err != nil {
					j.Logger.Error("Failed to remove container", "id", c.ID[:12], "error", err)
				} else {
					removedCount++
				}
			}
		}
	}

	if removedCount > 0 {
		j.Logger.Info("Janitor cleanup completed", "removed_count", removedCount)
	}

	return nil
}
