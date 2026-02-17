package orchestrator

import (
	"context"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

type Janitor struct {
	Client        DockerClient
	MaxAge        time.Duration
	DryRun        bool
	CheckInterval time.Duration
	Logger        *slog.Logger
}

func NewJanitor(logger *slog.Logger, client DockerClient, maxAge time.Duration, checkInterval time.Duration, dryRun bool) *Janitor {
	return &Janitor{
		Client:        client,
		MaxAge:        maxAge,
		DryRun:        dryRun,
		CheckInterval: checkInterval,
		Logger:        logger,
	}
}

func (j *Janitor) Run(ctx context.Context) {
	j.Logger.Info("Starting Janitor", "max_age", j.MaxAge, "dry_run", j.DryRun, "interval", j.CheckInterval)
	ticker := time.NewTicker(j.CheckInterval)
	defer ticker.Stop()

	// Run once immediately
	j.Cleanup(ctx)

	for {
		select {
		case <-ctx.Done():
			j.Logger.Info("Janitor shutting down...")
			return
		case <-ticker.C:
			j.Cleanup(ctx)
		}
	}
}

func (j *Janitor) Cleanup(ctx context.Context) {
	j.Logger.Debug("Janitor running cleanup...")

	// List containers created by recac-orchestrator
	args := filters.NewArgs()
	args.Add("label", "created-by=recac-orchestrator")

	opts := container.ListOptions{
		All:     true,
		Filters: args,
	}

	containers, err := j.Client.ListContainers(ctx, opts)
	if err != nil {
		j.Logger.Error("Janitor failed to list containers", "error", err)
		return
	}

	now := time.Now()
	removedCount := 0

	for _, c := range containers {
		shouldRemove := false
		reason := ""

		// Check label first
		if createdAtStr, ok := c.Labels["created-at"]; ok {
			createdAt, err := time.Parse(time.RFC3339, createdAtStr)
			if err != nil {
				j.Logger.Warn("Invalid created-at label", "container", c.ID, "label", createdAtStr)
			} else {
				if now.Sub(createdAt) > j.MaxAge {
					shouldRemove = true
					reason = "age > max_age"
				}
			}
		} else {
			// Fallback to container creation time
			createdAt := time.Unix(c.Created, 0)
			if now.Sub(createdAt) > j.MaxAge {
				shouldRemove = true
				reason = "age (creation_time) > max_age"
			}
		}

		if shouldRemove {
			j.remove(ctx, c.ID, reason)
			removedCount++
		}
	}

	if removedCount > 0 {
		j.Logger.Info("Janitor cleanup complete", "removed", removedCount)
	}
}

func (j *Janitor) remove(ctx context.Context, containerID string, reason string) {
	if j.DryRun {
		j.Logger.Info("Janitor would remove container", "id", containerID, "reason", reason)
		return
	}

	j.Logger.Info("Janitor removing container", "id", containerID, "reason", reason)
	if err := j.Client.RemoveContainer(ctx, containerID, true); err != nil {
		j.Logger.Error("Janitor failed to remove container", "id", containerID, "error", err)
	}
}
