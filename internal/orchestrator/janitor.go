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
	Client DockerClient
	Logger *slog.Logger
	MaxAge time.Duration
	DryRun bool
}

func NewJanitor(client DockerClient, logger *slog.Logger, maxAge time.Duration, dryRun bool) *Janitor {
	return &Janitor{
		Client: client,
		Logger: logger,
		MaxAge: maxAge,
		DryRun: dryRun,
	}
}

func (j *Janitor) Cleanup(ctx context.Context) error {
	j.Logger.Debug("Janitor starting cleanup", "max_age", j.MaxAge, "dry_run", j.DryRun)

	// Filter for containers created by orchestrator
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
		// c.Created is int64 unix timestamp
		created := time.Unix(c.Created, 0)
		if time.Since(created) > j.MaxAge {
			age := time.Since(created).Round(time.Second)
			if j.DryRun {
				j.Logger.Info("Would remove container", "id", c.ID[:12], "age", age, "names", c.Names)
			} else {
				j.Logger.Info("Removing container", "id", c.ID[:12], "age", age, "names", c.Names)
				if err := j.Client.RemoveContainer(ctx, c.ID, true); err != nil {
					j.Logger.Error("Failed to remove container", "id", c.ID, "error", err)
				} else {
					removedCount++
				}
			}
		}
	}

	if removedCount > 0 {
		j.Logger.Info("Janitor cleanup finished", "removed", removedCount, "total_found", len(containers))
	}
	return nil
}
