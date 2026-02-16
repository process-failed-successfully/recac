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
	Client   DockerClient
	Project  string
	MaxAge   time.Duration
	Interval time.Duration
	Logger   *slog.Logger
}

func NewJanitor(logger *slog.Logger, client DockerClient, project string, maxAge, interval time.Duration) *Janitor {
	return &Janitor{
		Client:   client,
		Project:  project,
		MaxAge:   maxAge,
		Interval: interval,
		Logger:   logger,
	}
}

func (j *Janitor) Run(ctx context.Context) {
	j.Logger.Info("Starting Janitor", "interval", j.Interval, "max_age", j.MaxAge)
	ticker := time.NewTicker(j.Interval)
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
	j.Logger.Debug("Running cleanup...")

	// List containers created by this project
	opts := container.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", fmt.Sprintf("created-by=%s", j.Project)),
		),
	}

	containers, err := j.Client.ListContainers(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	now := time.Now()
	removedCount := 0

	for _, c := range containers {
		// Calculate age based on Created timestamp (Unix timestamp)
		created := time.Unix(c.Created, 0)
		age := now.Sub(created)

		shouldRemove := false
		reason := ""

		if c.State == "exited" || c.State == "dead" {
			if age > j.MaxAge {
				shouldRemove = true
				reason = fmt.Sprintf("exited and age %v > max_age %v", age, j.MaxAge)
			}
		} else {
			// For running containers, be more conservative
			// e.g., 10x max age, or just warn?
			// The memory says: "removes containers labeled created-by=<project> that are either exited or have been running longer than the specified age."
			// This implies Strict MaxAge.
			// But that sounds dangerous if MaxAge is small (e.g. 1h).
			// If MaxAge is intended for "stale sessions", then maybe it's fine.
			// Let's stick to the memory description but add a log warning.
			if age > j.MaxAge {
				shouldRemove = true
				reason = fmt.Sprintf("running and age %v > max_age %v", age, j.MaxAge)
			}
		}

		if shouldRemove {
			j.Logger.Info("Removing container", "id", c.ID[:12], "reason", reason, "state", c.State)
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
