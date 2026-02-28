package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types/container"
)

// Janitor cleans up old containers created by the orchestrator.
type Janitor struct {
	client     DockerClient
	interval   time.Duration
	cleanupAge time.Duration
	dryRun     bool
	logger     *slog.Logger
}

// NewJanitor creates a new Janitor.
func NewJanitor(logger *slog.Logger, client DockerClient, interval time.Duration, cleanupAge time.Duration, dryRun bool) *Janitor {
	return &Janitor{
		client:     client,
		interval:   interval,
		cleanupAge: cleanupAge,
		dryRun:     dryRun,
		logger:     logger,
	}
}

// Start starts the janitor loop.
func (j *Janitor) Start(ctx context.Context) {
	j.logger.Info("Starting Janitor", "interval", j.interval, "age", j.cleanupAge, "dry_run", j.dryRun)
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			j.logger.Info("Stopping Janitor")
			return
		case <-ticker.C:
			if err := j.Cleanup(ctx); err != nil {
				j.logger.Error("Janitor cleanup failed", "error", err)
			}
		}
	}
}

// Cleanup performs a single cleanup run.
func (j *Janitor) Cleanup(ctx context.Context) error {
	containers, err := j.client.ListContainers(ctx, container.ListOptions{
		All: true, // We want stopped containers too
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	cutoff := time.Now().Add(-j.cleanupAge)
	count := 0

	for _, c := range containers {
		// Filter by label
		if val, ok := c.Labels["created-by"]; !ok || val != "recac-orchestrator" {
			continue
		}

		// Check state and age
		// We remove if it's explicitly exited/dead OR if it's older than cutoff
		createdAt := time.Unix(c.Created, 0)
		if c.State != "exited" && c.State != "dead" && createdAt.After(cutoff) {
			continue
		}

		// Cleanup
		id := c.ID
		if len(id) > 12 {
			id = id[:12]
		}

		workItem := c.Labels["work-item"]

		j.logger.Info("Janitor found reclaimable container", "id", id, "work_item", workItem, "created_at", createdAt)

		if !j.dryRun {
			if err := j.client.RemoveContainer(ctx, c.ID, true); err != nil {
				j.logger.Error("Failed to remove container", "id", id, "error", err)
			} else {
				j.logger.Info("Removed container", "id", id)
				count++
			}
		} else {
			count++
		}
	}

	if count > 0 {
		j.logger.Info("Janitor run completed", "reclaimed", count)
	}
	return nil
}
