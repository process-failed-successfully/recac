package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

type Janitor struct {
	Client   DockerClient
	Logger   *slog.Logger
	DryRun   bool
	Interval time.Duration
	MaxAge   time.Duration
}

func NewJanitor(client DockerClient, logger *slog.Logger, interval, maxAge time.Duration, dryRun bool) *Janitor {
	return &Janitor{
		Client:   client,
		Logger:   logger,
		Interval: interval,
		MaxAge:   maxAge,
		DryRun:   dryRun,
	}
}

func (j *Janitor) Run(ctx context.Context) {
	j.Logger.Info("Starting Janitor service", "interval", j.Interval, "max_age", j.MaxAge, "dry_run", j.DryRun)
	ticker := time.NewTicker(j.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			j.Logger.Info("Janitor stopping...")
			return
		case <-ticker.C:
			if err := j.Cleanup(ctx); err != nil {
				j.Logger.Error("Janitor cleanup failed", "error", err)
			}
		}
	}
}

func (j *Janitor) Cleanup(ctx context.Context) error {
	j.Logger.Debug("Janitor running cleanup...")

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
		// c.Created is unix timestamp (int64)
		createdTime := time.Unix(c.Created, 0)
		age := time.Since(createdTime)

		if age > j.MaxAge {
			j.Logger.Info("Found expired container", "id", c.ID[:12], "age", age, "names", c.Names)

			if !j.DryRun {
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
		j.Logger.Info("Janitor cleanup completed", "removed_count", removedCount)
	} else {
		j.Logger.Debug("Janitor cleanup completed", "removed_count", 0)
	}

	return nil
}
