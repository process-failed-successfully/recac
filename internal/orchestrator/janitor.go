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
	Client       DockerClient
	MaxAge       time.Duration
	PollInterval time.Duration
	Logger       *slog.Logger
}

func NewJanitor(client DockerClient, maxAge time.Duration, pollInterval time.Duration, logger *slog.Logger) *Janitor {
	return &Janitor{
		Client:       client,
		MaxAge:       maxAge,
		PollInterval: pollInterval,
		Logger:       logger,
	}
}

func (j *Janitor) Run(ctx context.Context) {
	j.Logger.Info("Starting Janitor", "max_age", j.MaxAge, "interval", j.PollInterval)
	ticker := time.NewTicker(j.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			j.Logger.Info("Janitor shutting down...")
			return
		case <-ticker.C:
			if err := j.Cleanup(ctx); err != nil {
				j.Logger.Error("Janitor cleanup failed", "error", err)
			}
		}
	}
}

func (j *Janitor) Cleanup(ctx context.Context) error {
	j.Logger.Debug("Running Janitor cleanup")

	// Filter by label
	args := filters.NewArgs()
	args.Add("label", "created-by=recac-orchestrator")

	// Get ALL containers (running and exited)
	opts := container.ListOptions{
		All:     true,
		Filters: args,
	}

	containers, err := j.Client.ListContainers(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	now := time.Now().Unix()
	removedCount := 0

	for _, c := range containers {
		// Check age
		age := time.Duration(now-c.Created) * time.Second

		if age > j.MaxAge {
			// Avoid panic if ID is too short
			id := c.ID
			if len(id) > 12 {
				id = id[:12]
			}
			j.Logger.Info("Removing stale container", "id", id, "age", age, "names", c.Names)
			if err := j.Client.RemoveContainer(ctx, c.ID, true); err != nil {
				j.Logger.Error("Failed to remove container", "id", id, "error", err)
			} else {
				removedCount++
			}
		}
	}

	if removedCount > 0 {
		j.Logger.Info("Janitor cleanup complete", "removed", removedCount)
	}
	return nil
}
