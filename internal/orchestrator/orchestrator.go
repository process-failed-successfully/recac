package orchestrator

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"
)

type Orchestrator struct {
	Poller       Poller
	Spawner      Spawner
	PollInterval time.Duration
	wg           sync.WaitGroup

	mu            sync.RWMutex
	startTime     time.Time
	lastPoll      time.Time
	lastPollItems int
	activeSpawns  int
	totalSpawns   int
	activeJobs    map[string]JobInfo
}

type JobInfo struct {
	ID        string    `json:"id"`
	Summary   string    `json:"summary"`
	StartTime time.Time `json:"start_time"`
	Status    string    `json:"status"`
	WorkItem  WorkItem  `json:"work_item"`
}

type Status struct {
	PollInterval  string    `json:"poll_interval"`
	Uptime        string    `json:"uptime"`
	LastPoll      time.Time `json:"last_poll"`
	LastPollItems int       `json:"last_poll_items"`
	ActiveSpawns  int       `json:"active_spawns"`
	TotalSpawns   int       `json:"total_spawns"`
}

func New(poller Poller, spawner Spawner, pollInterval time.Duration) *Orchestrator {
	return &Orchestrator{
		Poller:       poller,
		Spawner:      spawner,
		PollInterval: pollInterval,
		activeJobs:   make(map[string]JobInfo),
	}
}

// GetActiveJobs returns the list of currently running jobs.
func (o *Orchestrator) GetActiveJobs() []JobInfo {
	o.mu.RLock()
	defer o.mu.RUnlock()

	jobs := make([]JobInfo, 0, len(o.activeJobs))
	for _, job := range o.activeJobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// GetJob returns the details of a specific job.
func (o *Orchestrator) GetJob(id string) (JobInfo, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	job, exists := o.activeJobs[id]
	if !exists {
		return JobInfo{}, fmt.Errorf("job %s not found", id)
	}
	return job, nil
}

// CancelJob cancels a running job.
func (o *Orchestrator) CancelJob(ctx context.Context, jobID string) error {
	// Attempt to cancel via Spawner.
	// We don't need to manually remove from activeJobs map because the Spawner.Spawn()
	// call in Run()'s goroutine will return an error (or finish) upon cancellation,
	// triggering the deferred cleanup.
	return o.Spawner.Cancel(ctx, jobID)
}

// GetLogs returns the logs for a specific job ID.
func (o *Orchestrator) GetLogs(ctx context.Context, jobID string) (io.ReadCloser, error) {
	return o.Spawner.GetLogs(ctx, jobID)
}

// GetStatus returns the current status of the orchestrator.
func (o *Orchestrator) GetStatus() Status {
	o.mu.RLock()
	defer o.mu.RUnlock()

	uptime := "Not started"
	if !o.startTime.IsZero() {
		uptime = time.Since(o.startTime).String()
	}

	return Status{
		PollInterval:  o.PollInterval.String(),
		Uptime:        uptime,
		LastPoll:      o.lastPoll,
		LastPollItems: o.lastPollItems,
		ActiveSpawns:  o.activeSpawns,
		TotalSpawns:   o.totalSpawns,
	}
}

// DryRun polls for work once and returns the items without spawning.
func (o *Orchestrator) DryRun(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	logger.Info("Starting Dry Run (one-off poll)")
	items, err := o.Poller.Poll(ctx, logger)
	if err != nil {
		return nil, fmt.Errorf("dry run poll failed: %w", err)
	}
	return items, nil
}

// Verify checks the connectivity of the Poller and Spawner.
func (o *Orchestrator) Verify(ctx context.Context, logger *slog.Logger) error {
	logger.Info("Starting Verification Check")

	// Check Poller
	logger.Info("Checking Poller connectivity...")
	if err := o.Poller.Ping(ctx); err != nil {
		logger.Error("Poller check failed", "error", err)
		return fmt.Errorf("poller check failed: %w", err)
	}
	logger.Info("Poller check passed")

	// Check Spawner
	logger.Info("Checking Spawner connectivity...")
	if err := o.Spawner.Ping(ctx); err != nil {
		logger.Error("Spawner check failed", "error", err)
		return fmt.Errorf("spawner check failed: %w", err)
	}
	logger.Info("Spawner check passed")

	logger.Info("Verification Successful: All systems go!")
	return nil
}

// SubmitJob manually submits a work item to the orchestrator.
func (o *Orchestrator) SubmitJob(ctx context.Context, item WorkItem, logger *slog.Logger) error {
	return o.processWorkItem(ctx, item, logger)
}

func (o *Orchestrator) processWorkItem(ctx context.Context, item WorkItem, logger *slog.Logger) error {
	o.mu.Lock()
	if _, exists := o.activeJobs[item.ID]; exists {
		o.mu.Unlock()
		return fmt.Errorf("job %s is already active", item.ID)
	}

	o.activeSpawns++
	o.totalSpawns++
	job := JobInfo{
		ID:        item.ID,
		Summary:   item.Summary,
		StartTime: time.Now(),
		Status:    "Spawning",
		WorkItem:  item,
	}
	o.activeJobs[item.ID] = job
	o.mu.Unlock()

	o.wg.Add(1)
	go o.spawnWorker(ctx, item, logger)
	return nil
}

func (o *Orchestrator) spawnWorker(ctx context.Context, item WorkItem, logger *slog.Logger) {
	defer o.wg.Done()
	defer func() {
		o.mu.Lock()
		o.activeSpawns--
		delete(o.activeJobs, item.ID)
		o.mu.Unlock()
	}()

	logger.Info("Spawning agent for item", "id", item.ID)

	if err := o.Spawner.Spawn(ctx, item); err != nil {
		logger.Error("Failed to spawn agent", "id", item.ID, "error", err)
		// Update status to Failed
		_ = o.Poller.UpdateStatus(ctx, item, "Failed", fmt.Sprintf("Failed to spawn agent: %v", err))
	} else {
		// Success? K8s Jobs are fire-and-forget from Spawner perspective usually,
		// but status updates might happen asynchronously.
		// For now, Spawn() implies "Started".
		logger.Info("Agent spawned successfully", "id", item.ID)
	}
}

// Run starts the orchestration loop
func (o *Orchestrator) Run(ctx context.Context, logger *slog.Logger) error {
	o.mu.Lock()
	o.startTime = time.Now()
	o.mu.Unlock()

	logger.Info("Starting Orchestrator", "interval", o.PollInterval)
	ticker := time.NewTicker(o.PollInterval)
	defer ticker.Stop()

	poll := func() {
		// Poll for work
		logger.Debug("Polling for work...")
		items, err := o.Poller.Poll(ctx, logger)

		o.mu.Lock()
		o.lastPoll = time.Now()
		if err == nil {
			o.lastPollItems = len(items)
		}
		o.mu.Unlock()

		if err != nil {
			logger.Error("Failed to poll for work", "error", err)
			return
		}

		if len(items) == 0 {
			return
		}

		logger.Info("Found work items", "count", len(items))

		for _, item := range items {
			if err := o.processWorkItem(ctx, item, logger); err != nil {
				// Log duplication as info, but real errors as errors
				if err.Error() == fmt.Sprintf("job %s is already active", item.ID) {
					logger.Info("Job already active, skipping", "id", item.ID)
				} else {
					logger.Error("Failed to process work item", "id", item.ID, "error", err)
				}
			}
		}
	}

	// Initial poll
	poll()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Orchestrator shutting down...")
			o.wg.Wait()
			return ctx.Err()
		case <-ticker.C:
			poll()
		}
	}
}
