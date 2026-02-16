package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

// CleanupOrphanedContainers removes Docker containers created by recac-orchestrator
// that are older than the specified ageThreshold (or all if zero).
func CleanupOrphanedContainers(ctx context.Context, client DockerClient, logger *slog.Logger, ageThreshold time.Duration) error {
	logger.Info("Starting cleanup of orphaned containers", "age_threshold", ageThreshold)

	// Filter by label
	args := filters.NewArgs()
	args.Add("label", "created-by=recac-orchestrator")

	containers, err := client.ListContainers(ctx, container.ListOptions{
		All:     true,
		Filters: args,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	logger.Info("Found containers to check", "count", len(containers))

	removedCount := 0
	for _, c := range containers {
		// Check Age
		created := time.Unix(c.Created, 0)
		age := time.Since(created)

		if ageThreshold > 0 && age < ageThreshold {
			continue
		}

		id := c.ID
		if len(id) > 12 {
			id = id[:12]
		}
		logger.Info("Removing orphaned container", "id", id, "created", created, "age", age)

		// Force remove
		if err := client.RemoveContainer(ctx, c.ID, true); err != nil {
			logger.Error("Failed to remove container", "id", id, "error", err)
			continue
		}
		removedCount++
	}

	logger.Info("Cleanup completed", "removed_count", removedCount)
	return nil
}
