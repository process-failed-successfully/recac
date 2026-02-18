package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// JanitorClient defines the interface for cleaning up resources.
type JanitorClient interface {
	// ListCandidates returns resources managed by the orchestrator (e.g. filtered by label).
	ListCandidates(ctx context.Context) ([]Candidate, error)
	// Remove deletes the resource.
	Remove(ctx context.Context, id string) error
}

// Candidate represents a resource that might be cleaned up.
type Candidate struct {
	ID        string
	Name      string
	WorkItem  string
	CreatedAt time.Time
	Labels    map[string]string
}

// Janitor cleans up old resources created by the orchestrator.
type Janitor struct {
	client     JanitorClient
	interval   time.Duration
	cleanupAge time.Duration
	dryRun     bool
	logger     *slog.Logger
}

// NewJanitor creates a new Janitor.
func NewJanitor(logger *slog.Logger, client JanitorClient, interval time.Duration, cleanupAge time.Duration, dryRun bool) *Janitor {
	return &Janitor{
		client:     client,
		interval:   interval,
		cleanupAge: cleanupAge,
		dryRun:     dryRun,
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

// Cleanup performs a single cleanup run.
func (j *Janitor) Cleanup(ctx context.Context) error {
	candidates, err := j.client.ListCandidates(ctx)
	if err != nil {
		return fmt.Errorf("failed to list candidates: %w", err)
	}

	cutoff := time.Now().Add(-j.cleanupAge)
	count := 0

	for _, c := range candidates {
		// Filter by label (redundant check if client filters, but safe)
		if val, ok := c.Labels["created-by"]; !ok || val != "recac-orchestrator" {
			continue
		}

		// Check age
		if c.CreatedAt.After(cutoff) {
			continue
		}

		// Cleanup
		id := c.ID
		if len(id) > 12 {
			id = id[:12]
		}

		workItem := c.Labels["work-item"]

		j.logger.Info("Janitor found reclaimable resource", "id", id, "work_item", workItem, "created_at", c.CreatedAt)

		if !j.dryRun {
			if err := j.client.Remove(ctx, c.ID); err != nil {
				j.logger.Error("Failed to remove resource", "id", id, "error", err)
			} else {
				j.logger.Info("Removed resource", "id", id)
				count++
			}
		} else {
			count++
		}
	}

	if count > 0 {
		j.logger.Info("Janitor run completed", "reclaimed", count)
	}
	return nil
}
