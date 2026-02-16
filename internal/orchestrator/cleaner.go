package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

// Cleaner handles cleanup of stale resources.
type Cleaner struct {
	Client  DockerClient
	Project string
	Logger  *slog.Logger
}

// NewCleaner creates a new Cleaner instance.
func NewCleaner(logger *slog.Logger, client DockerClient, project string) *Cleaner {
	return &Cleaner{
		Client:  client,
		Project: project,
		Logger:  logger,
	}
}

// Cleanup removes containers created by the orchestrator that are older than maxAge.
// Returns the list of removed container IDs.
func (c *Cleaner) Cleanup(ctx context.Context, maxAge time.Duration) ([]string, error) {
	c.Logger.Info("Starting cleanup", "project", c.Project, "max_age", maxAge)

	// Filter by label
	// Note: We construct filters manually or passed via ListOptions.
	// DockerClient.ListContainers takes container.ListOptions.
	opts := container.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", fmt.Sprintf("created-by=%s", c.Project)),
		),
	}

	containers, err := c.Client.ListContainers(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	c.Logger.Info("Found containers to check", "count", len(containers))

	var removed []string
	now := time.Now()

	for _, cont := range containers {
		// Check age
		// Created is unix timestamp
		created := time.Unix(cont.Created, 0)
		age := now.Sub(created)

		if age > maxAge {
			shortID := cont.ID
			if len(shortID) > 12 {
				shortID = shortID[:12]
			}
			c.Logger.Info("Removing stale container", "id", shortID, "age", age, "status", cont.Status)
			if err := c.Client.RemoveContainer(ctx, cont.ID, true); err != nil {
				c.Logger.Error("Failed to remove container", "id", cont.ID, "error", err)
				// Continue with others
				continue
			}
			removed = append(removed, cont.ID)
		} else {
			shortID := cont.ID
			if len(shortID) > 12 {
				shortID = shortID[:12]
			}
			c.Logger.Debug("Skipping container (too young)", "id", shortID, "age", age)
		}
	}

	c.Logger.Info("Cleanup completed", "removed", len(removed))
	return removed, nil
}
