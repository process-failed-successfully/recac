package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

// Cleaner handles cleanup of old resources.
type Cleaner struct {
	Client DockerClient
	Logger *slog.Logger
}

// NewCleaner creates a new Cleaner.
func NewCleaner(logger *slog.Logger, client DockerClient) *Cleaner {
	return &Cleaner{
		Client: client,
		Logger: logger,
	}
}

// Cleanup removes containers created by the orchestrator that are older than maxAge.
func (c *Cleaner) Cleanup(ctx context.Context, maxAge time.Duration, dryRun bool) ([]string, error) {
	c.Logger.Info("Starting cleanup", "max_age", maxAge, "dry_run", dryRun)

	// List containers with label "created-by=recac-orchestrator"
	opts := container.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", "created-by=recac-orchestrator"),
		),
	}

	containers, err := c.Client.ListContainers(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var removed []string
	now := time.Now()

	for _, cont := range containers {
		// Created is int64 unix timestamp
		createdAt := time.Unix(cont.Created, 0)
		age := now.Sub(createdAt)

		if age > maxAge {
			c.Logger.Info("Found old container", "id", cont.ID, "age", age, "names", cont.Names)
			if !dryRun {
				if err := c.Client.RemoveContainer(ctx, cont.ID, true); err != nil {
					c.Logger.Error("Failed to remove container", "id", cont.ID, "error", err)
				} else {
					c.Logger.Info("Removed container", "id", cont.ID)
					removed = append(removed, cont.ID)
				}
			} else {
				removed = append(removed, cont.ID)
			}
		}
	}

	return removed, nil
}
