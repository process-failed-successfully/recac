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

// JanitorDockerClient defines the subset of Docker API methods used by the Janitor.
type JanitorDockerClient interface {
	ListContainers(ctx context.Context, options container.ListOptions) ([]types.Container, error)
	RemoveContainer(ctx context.Context, containerID string, force bool) error
}

// Janitor periodically cleans up stale containers.
type Janitor struct {
	Client  JanitorDockerClient
	Project string
	MaxAge  time.Duration
	Cleanup bool
}

// NewJanitor creates a new Janitor.
func NewJanitor(client JanitorDockerClient, project string, maxAge time.Duration, cleanup bool) *Janitor {
	return &Janitor{
		Client:  client,
		Project: project,
		MaxAge:  maxAge,
		Cleanup: cleanup,
	}
}

// Run starts the Janitor loop.
func (j *Janitor) Run(ctx context.Context, logger *slog.Logger) {
	if !j.Cleanup {
		logger.Info("Janitor cleanup is disabled")
		return
	}

	logger.Info("Starting Janitor", "project", j.Project, "max_age", j.MaxAge)
	// Run cleanup immediately on start
	j.cleanup(ctx, logger)

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Janitor shutting down...")
			return
		case <-ticker.C:
			j.cleanup(ctx, logger)
		}
	}
}

func (j *Janitor) cleanup(ctx context.Context, logger *slog.Logger) {
	logger.Debug("Janitor running cleanup...")

	opts := container.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", fmt.Sprintf("created-by=%s", j.Project)),
		),
	}

	containers, err := j.Client.ListContainers(ctx, opts)
	if err != nil {
		logger.Error("Janitor failed to list containers", "error", err)
		return
	}

	for _, c := range containers {
		shouldRemove := false
		reason := ""

		if c.State == "exited" || c.State == "dead" {
			shouldRemove = true
			reason = fmt.Sprintf("state is %s", c.State)
		} else if j.MaxAge > 0 {
			// Check creation time
			created := time.Unix(c.Created, 0)
			if time.Since(created) > j.MaxAge {
				shouldRemove = true
				reason = fmt.Sprintf("running longer than %s", j.MaxAge)
			}
		}

		if shouldRemove {
			// Safety check: Avoid panic on short ID
			id := c.ID
			if len(id) > 12 {
				id = id[:12]
			}
			logger.Info("Janitor removing container", "id", id, "reason", reason)
			// Force remove to kill if running
			if err := j.Client.RemoveContainer(ctx, c.ID, true); err != nil {
				logger.Error("Janitor failed to remove container", "id", id, "error", err)
			}
		}
	}
}
