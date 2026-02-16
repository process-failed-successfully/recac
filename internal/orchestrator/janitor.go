package orchestrator

import (
	"context"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

// Janitor cleans up stale resources.
type Janitor struct {
	Client   DockerClient
	Logger   *slog.Logger
	Interval time.Duration
	MaxAge   time.Duration
}

// NewJanitor creates a new Janitor.
func NewJanitor(client DockerClient, logger *slog.Logger, interval, maxAge time.Duration) *Janitor {
	return &Janitor{
		Client:   client,
		Logger:   logger,
		Interval: interval,
		MaxAge:   maxAge,
	}
}

// Run starts the janitor loop.
func (j *Janitor) Run(ctx context.Context) {
	j.Logger.Info("Starting Janitor", "interval", j.Interval, "max_age", j.MaxAge)
	ticker := time.NewTicker(j.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			j.Logger.Info("Janitor stopping...")
			return
		case <-ticker.C:
			j.cleanup(ctx)
		}
	}
}

func (j *Janitor) cleanup(ctx context.Context) {
	j.Logger.Debug("Janitor: checking for stale containers")

	// Filter by label created-by=recac-orchestrator
	args := filters.NewArgs()
	args.Add("label", "created-by=recac-orchestrator")

	containers, err := j.Client.ListContainers(ctx, container.ListOptions{
		All:     true,
		Filters: args,
	})
	if err != nil {
		j.Logger.Error("Janitor: failed to list containers", "error", err)
		return
	}

	for _, c := range containers {
		// Calculate age
		// Created is int64 timestamp (unix seconds)
		created := time.Unix(c.Created, 0)
		age := time.Since(created)

		if age > j.MaxAge {
			// Check if ID is long enough to slice
			id := c.ID
			if len(id) > 12 {
				id = id[:12]
			}
			j.Logger.Info("Janitor: removing stale container", "id", id, "age", age, "names", c.Names)
			if err := j.Client.RemoveContainer(ctx, c.ID, true); err != nil {
				j.Logger.Error("Janitor: failed to remove container", "id", id, "error", err)
			} else {
				j.Logger.Info("Janitor: removed stale container", "id", id)
			}
		}
	}
}
