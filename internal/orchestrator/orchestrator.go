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
	completedJobs []JobInfo
	maxHistory    int
	paused        bool
	Persistence   Persistence
}

type JobInfo struct {
	ID        string    `json:"id"`
	Summary   string    `json:"summary"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
	WorkItem  WorkItem  `json:"work_item"`
}

type Status struct {
	PollInterval  string    `json:"poll_interval"`
	Uptime        string    `json:"uptime"`
	LastPoll      time.Time `json:"last_poll"`
	LastPollItems int       `json:"last_poll_items"`
	ActiveSpawns  int       `json:"active_spawns"`
	TotalSpawns   int       `json:"total_spawns"`
	Paused        bool      `json:"paused"`
}

func New(poller Poller, spawner Spawner, pollInterval time.Duration) *Orchestrator {
	return &Orchestrator{
		Poller:       poller,
		Spawner:      spawner,
		PollInterval: pollInterval,
		activeJobs:   make(map[string]JobInfo),
		maxHistory:   50, // Default history size
	}
}

// SetPersistence sets the persistence layer for the orchestrator.
func (o *Orchestrator) SetPersistence(p Persistence) {
	o.Persistence = p
}

// LoadHistory loads the job history from the persistence layer.
func (o *Orchestrator) LoadHistory(logger *slog.Logger) error {
	if o.Persistence == nil {
		return nil
	}

	jobs, err := o.Persistence.GetJobs(o.maxHistory)
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	// jobs are returned DESC (newest first).
	// completedJobs stores oldest first (append to end).
	// So we need to reverse the list to restore the correct order in memory.
	o.completedJobs = make([]JobInfo, 0, len(jobs))
	for i := len(jobs) - 1; i >= 0; i-- {
		o.completedJobs = append(o.completedJobs, jobs[i])
	}

	if logger != nil {
		logger.Info("Loaded job history", "count", len(o.completedJobs))
	}
	return nil
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
	if exists {
		return job, nil
	}

	// Check completed jobs (reverse order for most recent)
	for i := len(o.completedJobs) - 1; i >= 0; i-- {
		if o.completedJobs[i].ID == id {
			return o.completedJobs[i], nil
		}
	}

	// Check persistence if available
	if o.Persistence != nil {
		job, err := o.Persistence.GetJob(id)
		if err == nil {
			return *job, nil
		}
	}

	return JobInfo{}, fmt.Errorf("job %s not found", id)
}

// GetCompletedJobs returns the list of completed jobs.
func (o *Orchestrator) GetCompletedJobs() []JobInfo {
	o.mu.RLock()
	defer o.mu.RUnlock()

	// Return a copy to avoid race conditions
	jobs := make([]JobInfo, len(o.completedJobs))
	copy(jobs, o.completedJobs)
	return jobs
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
		Paused:        o.paused,
	}
}

// Pause pauses the orchestrator's polling loop.
func (o *Orchestrator) Pause() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.paused = true
}

// Resume resumes the orchestrator's polling loop.
func (o *Orchestrator) Resume() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.paused = false
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

// RetryJob resubmits a completed job from history.
func (o *Orchestrator) RetryJob(ctx context.Context, jobID string, logger *slog.Logger) error {
	o.mu.RLock()
	// 1. Check if active
	if _, exists := o.activeJobs[jobID]; exists {
		o.mu.RUnlock()
		return fmt.Errorf("job %s is already active", jobID)
	}

	// 2. Check history
	var workItem WorkItem
	found := false
	for i := len(o.completedJobs) - 1; i >= 0; i-- {
		if o.completedJobs[i].ID == jobID {
			workItem = o.completedJobs[i].WorkItem
			found = true
			break
		}
	}
	o.mu.RUnlock()

	if !found {
		return fmt.Errorf("job %s not found in history", jobID)
	}

	// 3. Resubmit
	logger.Info("Retrying job", "id", jobID)
	return o.processWorkItem(ctx, workItem, logger)
}

// RetryFailedJobs resubmits all failed jobs from history.
func (o *Orchestrator) RetryFailedJobs(ctx context.Context, logger *slog.Logger) (int, error) {
	o.mu.RLock()
	var toRetry []WorkItem
	for _, job := range o.completedJobs {
		if job.Status == "Failed" {
			// Check if already active
			if _, active := o.activeJobs[job.ID]; !active {
				toRetry = append(toRetry, job.WorkItem)
			}
		}
	}
	o.mu.RUnlock()

	count := 0
	for _, item := range toRetry {
		logger.Info("Retrying failed job", "id", item.ID)
		if err := o.processWorkItem(ctx, item, logger); err == nil {
			count++
		} else {
			logger.Error("Failed to retry job", "id", item.ID, "error", err)
		}
	}
	return count, nil
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

	var spawnErr error
	defer func() {
		o.mu.Lock()
		// Move to history
		if job, ok := o.activeJobs[item.ID]; ok {
			job.EndTime = time.Now()
			if spawnErr != nil {
				job.Status = "Failed"
				job.Error = spawnErr.Error()
			} else {
				job.Status = "Completed"
			}
			o.addToHistory(job, logger)
		}

		o.activeSpawns--
		delete(o.activeJobs, item.ID)
		o.mu.Unlock()
	}()

	logger.Info("Spawning agent for item", "id", item.ID)

	if err := o.Spawner.Spawn(ctx, item); err != nil {
		spawnErr = err
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

func (o *Orchestrator) addToHistory(job JobInfo, logger *slog.Logger) {
	o.completedJobs = append(o.completedJobs, job)
	if len(o.completedJobs) > o.maxHistory {
		// Remove oldest
		o.completedJobs = o.completedJobs[1:]
	}

	if o.Persistence != nil {
		if err := o.Persistence.SaveJob(job); err != nil {
			if logger != nil {
				logger.Error("Failed to persist job history", "job", job.ID, "error", err)
			}
		}
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
			o.mu.RLock()
			paused := o.paused
			o.mu.RUnlock()

			if !paused {
				poll()
			} else {
				logger.Debug("Orchestrator is paused, skipping poll")
			}
		}
	}
}
