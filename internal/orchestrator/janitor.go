package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

// Janitor cleans up orphaned containers.
type Janitor struct {
	Client      DockerClient
	Logger      *slog.Logger
	ProjectName string
	Interval    time.Duration
	MaxAge      time.Duration
}

// NewJanitor creates a new Janitor.
func NewJanitor(logger *slog.Logger, client DockerClient, projectName string, interval, maxAge time.Duration) *Janitor {
	return &Janitor{
		Client:      client,
		Logger:      logger,
		ProjectName: projectName,
		Interval:    interval,
		MaxAge:      maxAge,
	}
}

// Start starts the janitor loop. It blocks until context is done.
func (j *Janitor) Start(ctx context.Context) {
	j.Logger.Info("Starting Janitor", "interval", j.Interval, "max_age", j.MaxAge, "project", j.ProjectName)
	ticker := time.NewTicker(j.Interval)
	defer ticker.Stop()

	// Run once immediately
	if err := j.Cleanup(ctx); err != nil {
		j.Logger.Error("Janitor cleanup failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			j.Logger.Info("Stopping Janitor")
			return
		case <-ticker.C:
			if err := j.Cleanup(ctx); err != nil {
				j.Logger.Error("Janitor cleanup failed", "error", err)
			}
		}
	}
}

// Cleanup performs a single cleanup pass.
func (j *Janitor) Cleanup(ctx context.Context) error {
	j.Logger.Debug("Janitor running cleanup")

	// List containers created by this project
	filtersArgs := filters.NewArgs()
	filtersArgs.Add("label", fmt.Sprintf("created-by=%s", j.ProjectName))

	containers, err := j.Client.ListContainers(ctx, container.ListOptions{
		Filters: filtersArgs,
		All:     true, // List running and stopped
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	now := time.Now()
	removedCount := 0

	for _, c := range containers {
		shouldRemove := false
		reason := ""

		// Check Age for ALL containers to prevent race conditions with recently finished jobs
		// c.Created is unix timestamp
		created := time.Unix(c.Created, 0)
		age := now.Sub(created)

		if age > j.MaxAge {
			shouldRemove = true
			reason = fmt.Sprintf("age %s > max %s", age, j.MaxAge)
		} else if c.State == "dead" {
			// Always remove dead containers (infrastructure error)
			shouldRemove = true
			reason = "state is dead"
		}

		if shouldRemove {
			j.Logger.Info("Janitor removing container", "id", c.ID, "reason", reason, "names", c.Names)
			if err := j.Client.RemoveContainer(ctx, c.ID, true); err != nil {
				j.Logger.Warn("Janitor failed to remove container", "id", c.ID, "error", err)
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
