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

// JanitorConfig holds configuration for the Janitor.
type JanitorConfig struct {
	MaxAge   time.Duration
	Interval time.Duration
	DryRun   bool
}

// Janitor cleans up old containers created by the orchestrator.
type Janitor struct {
	client DockerClient
	config JanitorConfig
	logger *slog.Logger
}

// NewJanitor creates a new Janitor instance.
func NewJanitor(client DockerClient, config JanitorConfig, logger *slog.Logger) *Janitor {
	return &Janitor{
		client: client,
		config: config,
		logger: logger,
	}
}

// Start starts the Janitor loop in the background.
func (j *Janitor) Start(ctx context.Context) {
	// Initial cleanup
	go func() {
		if err := j.cleanup(ctx); err != nil {
			j.logger.Error("Initial Janitor cleanup failed", "error", err)
		}
	}()

	go func() {
		ticker := time.NewTicker(j.config.Interval)
		defer ticker.Stop()

		j.logger.Info("Janitor started", "interval", j.config.Interval, "max_age", j.config.MaxAge, "dry_run", j.config.DryRun)

		for {
			select {
			case <-ctx.Done():
				j.logger.Info("Janitor stopped")
				return
			case <-ticker.C:
				if err := j.cleanup(ctx); err != nil {
					j.logger.Error("Janitor cleanup failed", "error", err)
				}
			}
		}
	}()
}

func (j *Janitor) cleanup(ctx context.Context) error {
	j.logger.Debug("Janitor running cleanup")

	// Filter for containers created by recac-orchestrator
	listOpts := container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", "created-by=recac-orchestrator")),
	}

	containers, err := j.client.ListContainers(ctx, listOpts)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	now := time.Now()
	removedCount := 0

	for _, c := range containers {
		// Determine creation time
		var createdAt time.Time

		// Try to parse from label first
		if val, ok := c.Labels["created-at"]; ok {
			parsed, err := time.Parse(time.RFC3339, val)
			if err == nil {
				createdAt = parsed
			}
		}

		// Fallback to container Created field (Unix timestamp)
		if createdAt.IsZero() {
			createdAt = time.Unix(c.Created, 0)
		}

		age := now.Sub(createdAt)
		if age > j.config.MaxAge {
			cid := c.ID
			if len(cid) > 12 {
				cid = cid[:12]
			}
			j.logger.Info("Found old container", "id", cid, "age", age, "names", c.Names)

			if !j.config.DryRun {
				// Force remove
				if err := j.client.RemoveContainer(ctx, c.ID, true); err != nil {
					j.logger.Error("Failed to remove container", "id", cid, "error", err)
				} else {
					j.logger.Info("Removed container", "id", cid)
					removedCount++
				}
			} else {
				j.logger.Info("Dry run: would remove container", "id", cid)
			}
		}
	}

	if removedCount > 0 {
		j.logger.Info("Janitor cleanup complete", "removed", removedCount)
	}

	return nil
}

// Needed to avoid unused import error if types is not used explicitly in ListContainers return type
var _ types.Container
