package orchestrator

import (
	"context"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

// Janitor is a service that periodically cleans up old containers created by the orchestrator.
type Janitor struct {
	Client   DockerClient
	Logger   *slog.Logger
	Interval time.Duration
	MaxAge   time.Duration
	DryRun   bool
}

// NewJanitor creates a new Janitor instance.
func NewJanitor(client DockerClient, logger *slog.Logger, interval, maxAge time.Duration, dryRun bool) *Janitor {
	return &Janitor{
		Client:   client,
		Logger:   logger,
		Interval: interval,
		MaxAge:   maxAge,
		DryRun:   dryRun,
	}
}

// Run starts the Janitor service loop.
func (j *Janitor) Run(ctx context.Context) {
	j.Logger.Info("Starting Janitor service", "interval", j.Interval, "max_age", j.MaxAge, "dry_run", j.DryRun)
	ticker := time.NewTicker(j.Interval)
	defer ticker.Stop()

	// Run once immediately
	j.cleanup(ctx)

	for {
		select {
		case <-ctx.Done():
			j.Logger.Info("Janitor shutting down...")
			return
		case <-ticker.C:
			j.cleanup(ctx)
		}
	}
}

func (j *Janitor) cleanup(ctx context.Context) {
	j.Logger.Debug("Janitor running cleanup...")

	// Filter by label
	opts := container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", "created-by=recac-orchestrator")),
	}

	containers, err := j.Client.ListContainers(ctx, opts)
	if err != nil {
		j.Logger.Error("Janitor failed to list containers", "error", err)
		return
	}

	now := time.Now()
	removedCount := 0

	for _, c := range containers {
		// types.Container has Created int64 (Unix timestamp)
		createdTime := time.Unix(c.Created, 0)
		age := now.Sub(createdTime)

		if age > j.MaxAge {
			j.Logger.Info("Janitor identifying container for removal", "id", c.ID[:12], "age", age, "created", createdTime)
			if !j.DryRun {
				if err := j.Client.RemoveContainer(ctx, c.ID, true); err != nil {
					j.Logger.Error("Janitor failed to remove container", "id", c.ID[:12], "error", err)
				} else {
					j.Logger.Info("Janitor removed container", "id", c.ID[:12])
					removedCount++
				}
			} else {
				j.Logger.Info("Janitor (DryRun) would remove container", "id", c.ID[:12])
			}
		}
	}

	if removedCount > 0 {
		j.Logger.Info("Janitor cleanup complete", "removed", removedCount)
	}
}
