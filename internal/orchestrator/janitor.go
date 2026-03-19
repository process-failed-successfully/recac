package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
)

// Janitor cleans up old containers and log files created by the orchestrator.
type Janitor struct {
	client     DockerClient
	interval   time.Duration
	cleanupAge time.Duration
	dryRun     bool
	logDir     string
	logger     *slog.Logger
}

// NewJanitor creates a new Janitor.
func NewJanitor(logger *slog.Logger, client DockerClient, interval time.Duration, cleanupAge time.Duration, dryRun bool, logDir string) *Janitor {
	return &Janitor{
		client:     client,
		interval:   interval,
		cleanupAge: cleanupAge,
		dryRun:     dryRun,
		logDir:     logDir,
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

// Cleanup performs a single cleanup run for both containers and logs.
func (j *Janitor) Cleanup(ctx context.Context) error {
	var errs []error

	if j.client != nil {
		if err := j.cleanupContainers(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if j.logDir != "" {
		if err := j.cleanupLogs(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("janitor cleanup encountered errors: %v", errs)
	}
	return nil
}

// cleanupContainers removes old reclaimable containers.
func (j *Janitor) cleanupContainers(ctx context.Context) error {
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
		j.logger.Info("Janitor container cleanup completed", "reclaimed", count)
	}
	return nil
}

// cleanupLogs removes old log files.
func (j *Janitor) cleanupLogs(ctx context.Context) error {
	entries, err := os.ReadDir(j.logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to clean up
		}
		return fmt.Errorf("failed to read log directory: %w", err)
	}

	cutoff := time.Now().Add(-j.cleanupAge)
	count := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".log.gz") && !strings.HasSuffix(name, ".log.gz.tmp") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			j.logger.Warn("Failed to get file info", "file", name, "error", err)
			continue
		}

		if info.ModTime().After(cutoff) {
			continue
		}

		path := filepath.Join(j.logDir, name)
		j.logger.Info("Janitor found old log file", "file", name, "mod_time", info.ModTime())

		if !j.dryRun {
			if err := os.Remove(path); err != nil {
				j.logger.Error("Failed to remove log file", "file", name, "error", err)
			} else {
				j.logger.Info("Removed log file", "file", name)
				count++
			}
		} else {
			count++
		}
	}

	if count > 0 {
		j.logger.Info("Janitor log cleanup completed", "reclaimed_files", count)
	}

	return nil
}
