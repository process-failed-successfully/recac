package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

// Janitor manages cleanup of stale containers.
type Janitor struct {
	Client          DockerClient
	Logger          *slog.Logger
	CleanupInterval time.Duration
	MaxAge          time.Duration
}

// NewJanitor creates a new Janitor.
func NewJanitor(client DockerClient, logger *slog.Logger, interval, maxAge time.Duration) *Janitor {
	return &Janitor{
		Client:          client,
		Logger:          logger,
		CleanupInterval: interval,
		MaxAge:          maxAge,
	}
}

// Run starts the janitor loop.
func (j *Janitor) Run(ctx context.Context) error {
	j.Logger.Info("Starting Janitor", "interval", j.CleanupInterval, "max_age", j.MaxAge)
	ticker := time.NewTicker(j.CleanupInterval)
	defer ticker.Stop()

	// Run once immediately
	if err := j.cleanup(ctx); err != nil {
		j.Logger.Error("Janitor cleanup failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			j.Logger.Info("Janitor shutting down...")
			return ctx.Err()
		case <-ticker.C:
			if err := j.cleanup(ctx); err != nil {
				j.Logger.Error("Janitor cleanup failed", "error", err)
			}
		}
	}
}

func (j *Janitor) cleanup(ctx context.Context) error {
	j.Logger.Debug("Running cleanup...")

	// List containers created by recac-orchestrator
	args := filters.NewArgs()
	args.Add("label", "created-by=recac-orchestrator")

	opts := container.ListOptions{
		All:     true,
		Filters: args,
	}

	containers, err := j.Client.ListContainers(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	j.Logger.Info("Found containers to check", "count", len(containers))

	removedCount := 0
	for _, c := range containers {
		// Check created-at label first (most reliable)
		createdAtStr, ok := c.Labels["created-at"]
		var createdTime time.Time
		if ok {
			// Try parsing RFC3339
			var err error
			createdTime, err = time.Parse(time.RFC3339, createdAtStr)
			if err != nil {
				// Try Unix timestamp just in case
				if ts, err := strconv.ParseInt(createdAtStr, 10, 64); err == nil {
					createdTime = time.Unix(ts, 0)
				}
			}
		}

		// Fallback to container Created timestamp
		if createdTime.IsZero() {
			createdTime = time.Unix(c.Created, 0)
		}

		age := time.Since(createdTime)
		if age > j.MaxAge {
			j.Logger.Info("Removing stale container", "id", c.ID[:12], "age", age, "work_item", c.Labels["work-item"])
			if err := j.Client.RemoveContainer(ctx, c.ID, true); err != nil {
				j.Logger.Error("Failed to remove container", "id", c.ID[:12], "error", err)
			} else {
				removedCount++
			}
		}
	}

	if removedCount > 0 {
		j.Logger.Info("Cleanup completed", "removed", removedCount)
	}

	return nil
}
