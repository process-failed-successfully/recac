package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

// Janitor cleans up old containers spawned by the orchestrator.
type Janitor struct {
	Client DockerClient
	Logger *slog.Logger
	DryRun bool
}

// NewJanitor creates a new Janitor.
func NewJanitor(client DockerClient, logger *slog.Logger, dryRun bool) *Janitor {
	return &Janitor{
		Client: client,
		Logger: logger,
		DryRun: dryRun,
	}
}

// Run starts the janitor loop.
func (j *Janitor) Run(ctx context.Context, interval time.Duration, maxAge time.Duration) {
	j.Logger.Info("Starting Janitor service", "interval", interval, "max_age", maxAge, "dry_run", j.DryRun)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run once immediately
	if err := j.Cleanup(ctx, maxAge); err != nil {
		j.Logger.Error("Janitor cleanup failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			j.Logger.Info("Janitor stopping")
			return
		case <-ticker.C:
			if err := j.Cleanup(ctx, maxAge); err != nil {
				j.Logger.Error("Janitor cleanup failed", "error", err)
			}
		}
	}
}

// Cleanup performs a single cleanup pass.
func (j *Janitor) Cleanup(ctx context.Context, maxAge time.Duration) error {
	j.Logger.Debug("Janitor: starting cleanup pass")

	// Filter for containers created by recac-orchestrator
	args := filters.NewArgs()
	args.Add("label", "created-by=recac-orchestrator")

	containers, err := j.Client.ListContainers(ctx, container.ListOptions{
		All:     true, // Include stopped containers
		Filters: args,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)
	count := 0
	removed := 0

	for _, c := range containers {
		count++
		// c.Created is unix timestamp
		created := time.Unix(c.Created, 0)

		if created.Before(cutoff) {
			j.Logger.Info("Janitor: Found expired container", "id", c.ID[:12], "created", created, "age", time.Since(created))

			if !j.DryRun {
				if err := j.Client.RemoveContainer(ctx, c.ID, true); err != nil {
					j.Logger.Error("Janitor: Failed to remove container", "id", c.ID[:12], "error", err)
				} else {
					j.Logger.Info("Janitor: Removed container", "id", c.ID[:12])
					removed++
				}
			} else {
				j.Logger.Info("Janitor: Dry-run, skipping removal", "id", c.ID[:12])
			}
		}
	}

	j.Logger.Debug("Janitor: cleanup pass completed", "found", count, "removed", removed)
	return nil
}
