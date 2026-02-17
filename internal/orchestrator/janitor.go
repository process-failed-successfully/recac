package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

// JanitorConfig holds configuration for the Janitor.
type JanitorConfig struct {
	MaxAge   time.Duration
	Interval time.Duration
	DryRun   bool
}

// Janitor cleans up old resources created by the Orchestrator.
type Janitor struct {
	Client DockerClient
	Config JanitorConfig
	Logger *slog.Logger
}

// NewJanitor creates a new Janitor.
func NewJanitor(logger *slog.Logger, client DockerClient, cfg JanitorConfig) *Janitor {
	return &Janitor{
		Client: client,
		Config: cfg,
		Logger: logger,
	}
}

// Run starts the Janitor loop. It blocks until context is cancelled.
func (j *Janitor) Run(ctx context.Context) {
	j.Logger.Info("Starting Janitor", "interval", j.Config.Interval, "max_age", j.Config.MaxAge, "dry_run", j.Config.DryRun)
	ticker := time.NewTicker(j.Config.Interval)
	defer ticker.Stop()

	// Run once immediately
	if err := j.Cleanup(ctx); err != nil {
		j.Logger.Error("Janitor cleanup failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			j.Logger.Info("Stopping Janitor")
			return
		case <-ticker.C:
			if err := j.Cleanup(ctx); err != nil {
				j.Logger.Error("Janitor cleanup failed", "error", err)
			}
		}
	}
}

// Cleanup performs a single cleanup cycle.
func (j *Janitor) Cleanup(ctx context.Context) error {
	j.Logger.Debug("Running cleanup cycle")

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

	j.Logger.Info("Found containers to check", "count", len(containers))

	now := time.Now()
	removedCount := 0

	for _, c := range containers {
		// Created is int64 timestamp (unix seconds)
		createdAt := time.Unix(c.Created, 0)
		age := now.Sub(createdAt)

		if age > j.Config.MaxAge {
			j.Logger.Info("Found old container", "id", c.ID[:12], "age", age, "names", c.Names, "created", createdAt)

			if !j.Config.DryRun {
				if err := j.Client.RemoveContainer(ctx, c.ID, true); err != nil {
					j.Logger.Error("Failed to remove container", "id", c.ID[:12], "error", err)
				} else {
					j.Logger.Info("Removed container", "id", c.ID[:12])
					removedCount++
				}
			} else {
				j.Logger.Info("Dry run: would remove container", "id", c.ID[:12])
			}
		}
	}

	if removedCount > 0 {
		j.Logger.Info("Cleanup complete", "removed", removedCount)
	}

	return nil
}
