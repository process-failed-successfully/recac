package orphan

import (
	"context"
	"log"
	"time"
)

// JobStatus represents the state of a job.
type JobStatus string

const (
	StatusPending   JobStatus = "Pending"
	StatusRunning   JobStatus = "Running"
	StatusCompleted JobStatus = "Completed"
	StatusFailed    JobStatus = "Failed"
)

// JobInfo contains metadata about a job.
type JobInfo struct {
	ID        string
	Status    JobStatus
	AgentID   string
	CreatedAt time.Time
}

// JobRegistry defines an interface for querying job states.
type JobRegistry interface {
	GetActiveJobs() ([]JobInfo, error)
	GetJobStatus(jobID string) (JobStatus, error)
}

// OrphanDetector scans for orphaned jobs.
type OrphanDetector struct {
	registry    JobRegistry
	scanInterval time.Duration
	pendingThreshold time.Duration
}

// NewOrphanDetector creates a new detector.
func NewOrphanDetector(registry JobRegistry, scanInterval, pendingThreshold time.Duration) *OrphanDetector {
	return &OrphanDetector{
		registry:    registry,
		scanInterval: scanInterval,
		pendingThreshold: pendingThreshold,
	}
}

// Scan continuously checks for orphaned jobs.
func (d *OrphanDetector) Scan(ctx context.Context) {
	ticker := time.NewTicker(d.scanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("OrphanDetector: stopping scan")
			return
		case <-ticker.C:
			if err := d.detectOrphans(); err != nil {
				log.Printf("OrphanDetector: scan error: %v", err)
			}
		}
	}
}

// detectOrphans identifies jobs without active agents or stuck in pending.
func (d *OrphanDetector) detectOrphans() error {
	jobs, err := d.registry.GetActiveJobs()
	if err != nil {
		return err
	}

	var orphans []JobInfo
	for _, job := range jobs {
		// Check for jobs without active agents
		if job.AgentID == "" && job.Status == StatusRunning {
			orphans = append(orphans, job)
			continue
		}

		// Check for jobs stuck in pending
		if job.Status == StatusPending && time.Since(job.CreatedAt) > d.pendingThreshold {
			orphans = append(orphans, job)
		}
	}

	if len(orphans) > 0 {
		log.Printf("OrphanDetector: found %d orphaned jobs", len(orphans))
		for _, job := range orphans {
			log.Printf("Orphaned job: ID=%s, Status=%s, AgentID=%s", job.ID, job.Status, job.AgentID)
		}
	}

	return nil
}
