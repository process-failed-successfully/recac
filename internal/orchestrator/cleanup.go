package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

// CleanerDockerClient defines the subset of Docker Client needed for cleanup.
type CleanerDockerClient interface {
	ListContainers(ctx context.Context, options container.ListOptions) ([]types.Container, error)
	RemoveContainer(ctx context.Context, containerID string, force bool) error
}

// CleanupContainers removes containers created by the orchestrator that are older than the specified duration.
func CleanupContainers(ctx context.Context, client CleanerDockerClient, olderThan time.Duration, dryRun bool, logger *slog.Logger) error {
	logger.Info("Starting cleanup", "older_than", olderThan, "dry_run", dryRun)

	// Filter for containers created by recac-orchestrator
	args := filters.NewArgs()
	args.Add("label", "created-by=recac-orchestrator")

	containers, err := client.ListContainers(ctx, container.ListOptions{
		Filters: args,
		All:     true, // List all containers (running and stopped)
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	cutoffTime := time.Now().Add(-olderThan)
	logger.Info("Found candidates", "count", len(containers), "cutoff", cutoffTime)

	removedCount := 0
	failedCount := 0

	for _, c := range containers {
		// c.Created is unix timestamp
		created := time.Unix(c.Created, 0)

		if created.Before(cutoffTime) {
			if dryRun {
				logger.Info("[DRY-RUN] Would remove container",
					"id", c.ID[:12],
					"name", getContainerName(c),
					"created", created,
					"status", c.Status)
				removedCount++ // Count as "would remove"
			} else {
				logger.Info("Removing container",
					"id", c.ID[:12],
					"name", getContainerName(c),
					"created", created,
					"status", c.Status)

				if err := client.RemoveContainer(ctx, c.ID, true); err != nil {
					logger.Error("Failed to remove container", "id", c.ID[:12], "error", err)
					failedCount++
				} else {
					removedCount++
				}
			}
		} else {
			logger.Debug("Skipping container (too new)",
				"id", c.ID[:12],
				"created", created)
		}
	}

	logger.Info("Cleanup complete", "processed", len(containers), "removed", removedCount, "failed", failedCount)
	return nil
}

func getContainerName(c types.Container) string {
	if len(c.Names) > 0 {
		return c.Names[0]
	}
	return ""
}
