package orchestrator

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type Orchestrator struct {
	Poller       Poller
	Spawner      Spawner
	PollInterval time.Duration
	wg           sync.WaitGroup

	mu                sync.RWMutex
	startTime         time.Time
	lastPoll          time.Time
	lastPollItems     int
	activeSpawns      int
	totalSpawns       int
	activeJobs        map[string]JobInfo
	completedJobs     []JobInfo
	pendingJobs       map[string]JobInfo
	maxHistory        int
	paused            bool
	draining          bool
	Persistence       Persistence
	forcePollCh       chan struct{}
	MaxConcurrentJobs int
	JobTimeout        time.Duration
	notifier          Notifier
	MaxRetries        int
	RetryDelay        time.Duration
	LogDir            string
	RequireApproval   bool

	eventChans map[chan []byte]struct{}
	eventMu    sync.RWMutex
}

var ErrAtCapacity = fmt.Errorf("orchestrator is at max capacity")
var ErrDraining = fmt.Errorf("orchestrator is draining and cannot accept new jobs")

type JobInfo struct {
	ID          string    `json:"id"`
	Summary     string    `json:"summary"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time,omitempty"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	WorkItem    WorkItem  `json:"work_item"`
	ThreadState string    `json:"thread_state,omitempty"`
	RetryCount  int       `json:"retry_count,omitempty"`
	RetryAfter  time.Time `json:"retry_after,omitempty"`
	Approved    bool      `json:"approved,omitempty"`
}

type Status struct {
	PollInterval      string    `json:"poll_interval"`
	Uptime            string    `json:"uptime"`
	LastPoll          time.Time `json:"last_poll"`
	LastPollItems     int       `json:"last_poll_items"`
	ActiveSpawns      int       `json:"active_spawns"`
	PendingJobs       int       `json:"pending_jobs"`
	TotalSpawns       int       `json:"total_spawns"`
	Paused            bool      `json:"paused"`
	Draining          bool      `json:"draining"`
	MaxConcurrentJobs int       `json:"max_concurrent_jobs"`
}

type Analytics struct {
	TotalJobs       int           `json:"total_jobs"`
	SuccessfulJobs  int           `json:"successful_jobs"`
	FailedJobs      int           `json:"failed_jobs"`
	CanceledJobs    int           `json:"canceled_jobs"`
	SuccessRate     float64       `json:"success_rate"`
	AverageDuration time.Duration `json:"average_duration"`
}

func New(poller Poller, spawner Spawner, pollInterval time.Duration) *Orchestrator {
	return &Orchestrator{
		Poller:       poller,
		Spawner:      spawner,
		PollInterval: pollInterval,
		activeJobs:   make(map[string]JobInfo),
		pendingJobs:  make(map[string]JobInfo),
		maxHistory:   50, // Default history size
		forcePollCh:  make(chan struct{}, 1),
		eventChans:   make(map[chan []byte]struct{}),
	}
}

// ForcePoll triggers an immediate poll cycle.
func (o *Orchestrator) ForcePoll() {
	select {
	case o.forcePollCh <- struct{}{}:
	default:
		// Already a poll pending
	}
}

// SetPersistence sets the persistence layer for the orchestrator.
func (o *Orchestrator) SetPersistence(p Persistence) {
	o.Persistence = p
}

// SetNotifier sets the notification manager for the orchestrator.
func (o *Orchestrator) SetNotifier(n Notifier) {
	o.notifier = n
}

// Subscribe returns a channel that receives orchestrator events.
func (o *Orchestrator) Subscribe() chan []byte {
	ch := make(chan []byte, 100) // Buffer to avoid blocking
	o.eventMu.Lock()
	defer o.eventMu.Unlock()
	o.eventChans[ch] = struct{}{}
	return ch
}

// Unsubscribe removes an event channel.
func (o *Orchestrator) Unsubscribe(ch chan []byte) {
	o.eventMu.Lock()
	defer o.eventMu.Unlock()
	if _, ok := o.eventChans[ch]; ok {
		delete(o.eventChans, ch)
		close(ch)
	}
}

// BroadcastEvent sends an event to all subscribers.
func (o *Orchestrator) BroadcastEvent(eventType string, data interface{}) {
	o.eventMu.RLock()
	defer o.eventMu.RUnlock()

	if len(o.eventChans) == 0 {
		return
	}

	event := map[string]interface{}{
		"event":     eventType,
		"data":      data,
		"timestamp": time.Now(),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return
	}

	for ch := range o.eventChans {
		select {
		case ch <- payload:
		default:
			// Drop event if subscriber is too slow
		}
	}
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

	jobs := make([]JobInfo, 0, len(o.activeJobs)+len(o.pendingJobs))
	for _, job := range o.activeJobs {
		jobs = append(jobs, job)
	}
	for _, job := range o.pendingJobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// GetPendingJobs returns the list of currently pending jobs.
func (o *Orchestrator) GetPendingJobs() []JobInfo {
	o.mu.RLock()
	defer o.mu.RUnlock()

	jobs := make([]JobInfo, 0, len(o.pendingJobs))
	for _, job := range o.pendingJobs {
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

	job, exists = o.pendingJobs[id]
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
	o.mu.Lock()
	if job, exists := o.pendingJobs[jobID]; exists {
		delete(o.pendingJobs, jobID)
		job.Status = "Canceled"
		job.EndTime = time.Now()
		job.Error = "Canceled by user"
		o.addToHistory(job, nil)
		o.mu.Unlock()
		o.BroadcastEvent("job_canceled", job)
		return nil
	}
	o.mu.Unlock()
	// Attempt to cancel via Spawner.
	// We don't need to manually remove from activeJobs map because the Spawner.Spawn()
	// call in Run()'s goroutine will return an error (or finish) upon cancellation,
	// triggering the deferred cleanup.
	return o.Spawner.Cancel(ctx, jobID)
}

// ClearPendingJobs cancels all jobs currently waiting for dependencies.
func (o *Orchestrator) ClearPendingJobs(ctx context.Context, logger *slog.Logger) int {
	o.mu.Lock()
	defer o.mu.Unlock()

	count := 0
	for id, job := range o.pendingJobs {
		delete(o.pendingJobs, id)
		job.Status = "Canceled"
		job.EndTime = time.Now()
		job.Error = "Canceled from pending queue"
		o.addToHistory(job, logger)
		count++
		o.BroadcastEvent("job_canceled", job)
	}

	if count > 0 && logger != nil {
		logger.Info("Cleared pending jobs", "count", count)
	}

	return count
}

// CancelJobsByTag cancels all running and pending jobs that have the specified tag.
func (o *Orchestrator) CancelJobsByTag(ctx context.Context, tag string, logger *slog.Logger) (int, error) {
	o.mu.Lock()
	var jobIDs []string
	lowerTag := strings.ToLower(tag)

	for id, job := range o.activeJobs {
		for _, t := range job.WorkItem.Tags {
			if strings.ToLower(t) == lowerTag {
				jobIDs = append(jobIDs, id)
				break
			}
		}
	}
	for id, job := range o.pendingJobs {
		for _, t := range job.WorkItem.Tags {
			if strings.ToLower(t) == lowerTag {
				jobIDs = append(jobIDs, id)
				break
			}
		}
	}
	o.mu.Unlock()

	if logger != nil && len(jobIDs) > 0 {
		logger.Info("Canceling jobs by tag", "tag", tag, "count", len(jobIDs))
	}

	count := 0
	var lastErr error
	for _, id := range jobIDs {
		if err := o.CancelJob(ctx, id); err != nil {
			lastErr = err
		} else {
			count++
		}
	}

	return count, lastErr
}

// CancelAllJobs cancels all running jobs.
func (o *Orchestrator) CancelAllJobs(ctx context.Context) (int, error) {
	// Get all active and pending job IDs first to avoid holding the lock during cancellation
	o.mu.Lock()
	var jobIDs []string
	for id := range o.activeJobs {
		jobIDs = append(jobIDs, id)
	}
	for id := range o.pendingJobs {
		jobIDs = append(jobIDs, id)
	}
	o.mu.Unlock()

	count := 0
	var lastErr error
	for _, id := range jobIDs {
		if err := o.CancelJob(ctx, id); err != nil {
			lastErr = err
		} else {
			count++
		}
	}

	return count, lastErr
}

// GetLogs returns the logs for a specific job ID.
func (o *Orchestrator) GetLogs(ctx context.Context, jobID string) (io.ReadCloser, error) {
	if o.LogDir != "" {
		safeID := filepath.Base(jobID)
		if safeID != "." && safeID != ".." && safeID != "/" && safeID != "\\" && !strings.Contains(safeID, "/") && !strings.Contains(safeID, "\\") {
			logPath := filepath.Join(o.LogDir, fmt.Sprintf("%s.log.gz", safeID))
			f, err := os.Open(logPath)
			if err == nil {
				gzReader, err := gzip.NewReader(f)
				if err == nil {
					return &gzipReadCloser{gzReader: gzReader, file: f}, nil
				}
				f.Close()
			}
		}
	}
	return o.Spawner.GetLogs(ctx, jobID)
}

type gzipReadCloser struct {
	gzReader *gzip.Reader
	file     *os.File
}

func (g *gzipReadCloser) Read(p []byte) (n int, err error) {
	return g.gzReader.Read(p)
}

func (g *gzipReadCloser) Close() error {
	err := g.gzReader.Close()
	if err2 := g.file.Close(); err == nil {
		err = err2
	}
	return err
}

// SetConcurrency sets the maximum number of concurrent jobs allowed.
func (o *Orchestrator) SetConcurrency(ctx context.Context, max int, logger *slog.Logger) {
	o.mu.Lock()
	oldMax := o.MaxConcurrentJobs
	o.MaxConcurrentJobs = max
	o.mu.Unlock()

	if logger != nil {
		logger.Info("Concurrency limit updated", "old", oldMax, "new", max)
	}

	// If we increased the limit or set to unlimited, we might have pending jobs that can now run
	if max == 0 || max > oldMax {
		o.evaluatePendingJobs(ctx, logger)
	}
}

// GetAnalytics calculates and returns analytics for the orchestrator's job history.
func (o *Orchestrator) GetAnalytics() Analytics {
	o.mu.RLock()
	var jobs []JobInfo
	if o.Persistence != nil {
		// Fetch up to 10,000 jobs from persistence to calculate stats
		if pJobs, err := o.Persistence.GetJobs(10000); err == nil {
			jobs = pJobs
		} else {
			jobs = o.completedJobs
		}
	} else {
		jobs = make([]JobInfo, len(o.completedJobs))
		copy(jobs, o.completedJobs)
	}
	o.mu.RUnlock()

	var stats Analytics
	var totalDuration time.Duration
	var durationCount int

	for _, job := range jobs {
		stats.TotalJobs++
		if job.Status == "Completed" {
			stats.SuccessfulJobs++
			if !job.EndTime.IsZero() && !job.StartTime.IsZero() {
				totalDuration += job.EndTime.Sub(job.StartTime)
				durationCount++
			}
		} else if job.Status == "Failed" || job.Status == "error" {
			stats.FailedJobs++
		} else if job.Status == "Canceled" {
			stats.CanceledJobs++
		}
	}

	if stats.TotalJobs > 0 {
		stats.SuccessRate = float64(stats.SuccessfulJobs) / float64(stats.TotalJobs) * 100.0
	}

	if durationCount > 0 {
		stats.AverageDuration = totalDuration / time.Duration(durationCount)
	}

	return stats
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
		PollInterval:      o.PollInterval.String(),
		Uptime:            uptime,
		LastPoll:          o.lastPoll,
		LastPollItems:     o.lastPollItems,
		ActiveSpawns:      o.activeSpawns,
		PendingJobs:       len(o.pendingJobs),
		TotalSpawns:       o.totalSpawns,
		Paused:            o.paused,
		Draining:          o.draining,
		MaxConcurrentJobs: o.MaxConcurrentJobs,
	}
}

// Drain sets the orchestrator to drain mode, stopping it from accepting new jobs.
func (o *Orchestrator) Drain(logger *slog.Logger) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.draining = true
	if logger != nil {
		logger.Info("Orchestrator is now draining")
	}
}

// Undrain removes the orchestrator from drain mode.
func (o *Orchestrator) Undrain(logger *slog.Logger) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.draining = false
	if logger != nil {
		logger.Info("Orchestrator has stopped draining")
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
	return o.processWorkItem(ctx, item, 0, logger)
}

// ApproveJob approves a job in the 'Pending Approval' state so it can be scheduled.
func (o *Orchestrator) ApproveJob(ctx context.Context, jobID string, logger *slog.Logger) error {
	o.mu.Lock()
	job, exists := o.pendingJobs[jobID]
	if !exists {
		o.mu.Unlock()
		// Check if active or completed to return a more specific error
		o.mu.RLock()
		if _, active := o.activeJobs[jobID]; active {
			o.mu.RUnlock()
			return fmt.Errorf("job %s is active and not pending approval", jobID)
		}
		for _, completed := range o.completedJobs {
			if completed.ID == jobID {
				o.mu.RUnlock()
				return fmt.Errorf("job %s is already completed and not pending approval", jobID)
			}
		}
		o.mu.RUnlock()
		return fmt.Errorf("job %s not found", jobID)
	}

	if job.Status != "Pending Approval" {
		o.mu.Unlock()
		return fmt.Errorf("job %s is not pending approval", jobID)
	}

	job.Status = "Pending"
	job.Approved = true
	o.pendingJobs[jobID] = job
	o.mu.Unlock()
	o.BroadcastEvent("job_approved", job)

	if logger != nil {
		logger.Info("Job approved", "id", jobID)
	}

	// Now that it's approved, evaluate pending jobs without the lock to avoid deadlock
	// We run it synchronously so tests see it schedule immediately.
	o.evaluatePendingJobs(ctx, logger)
	return nil
}

// UpdateJobPriority updates the priority of a job in the pending queue.
func (o *Orchestrator) UpdateJobPriority(ctx context.Context, jobID string, newPriority int, logger *slog.Logger) error {
	o.mu.Lock()
	job, exists := o.pendingJobs[jobID]
	if !exists {
		o.mu.Unlock()
		// Check if active or completed to return a more specific error
		o.mu.RLock()
		if _, active := o.activeJobs[jobID]; active {
			o.mu.RUnlock()
			return fmt.Errorf("job %s is already active and cannot have priority updated", jobID)
		}
		for _, completed := range o.completedJobs {
			if completed.ID == jobID {
				o.mu.RUnlock()
				return fmt.Errorf("job %s is already completed", jobID)
			}
		}
		o.mu.RUnlock()
		return fmt.Errorf("job %s not found in pending queue", jobID)
	}

	job.WorkItem.Priority = newPriority
	o.pendingJobs[jobID] = job
	o.mu.Unlock()
	o.BroadcastEvent("job_priority_updated", job)

	if logger != nil {
		logger.Info("Updated job priority", "jobID", jobID, "newPriority", newPriority)
	}

	o.evaluatePendingJobs(ctx, logger)
	return nil
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
	return o.processWorkItem(ctx, workItem, 0, logger)
}

// RetryFailedJobs resubmits all failed jobs from history.
func (o *Orchestrator) RetryFailedJobs(ctx context.Context, match string, logger *slog.Logger) (int, error) {
	var matcher *regexp.Regexp
	var err error
	if match != "" {
		matcher, err = regexp.Compile(match)
		if err != nil {
			return 0, fmt.Errorf("invalid retry match pattern: %w", err)
		}
	}

	o.mu.RLock()
	var toRetry []WorkItem
	for _, job := range o.completedJobs {
		if job.Status == "Failed" {
			if matcher != nil && !matcher.MatchString(job.Error) {
				continue
			}
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
		if err := o.processWorkItem(ctx, item, 0, logger); err == nil {
			count++
		} else {
			logger.Error("Failed to retry job", "id", item.ID, "error", err)
		}
	}
	return count, nil
}

func (o *Orchestrator) checkDependenciesMetLocked(dependsOn []string) (bool, string) {
	for _, dep := range dependsOn {
		met := false

		if _, ok := o.activeJobs[dep]; ok {
			return false, "" // Active, not met yet
		}
		if _, ok := o.pendingJobs[dep]; ok {
			return false, "" // Pending, not met yet
		}

		// Check memory history (newest first)
		for i := len(o.completedJobs) - 1; i >= 0; i-- {
			completed := o.completedJobs[i]
			if completed.ID == dep {
				if completed.Status == "Completed" {
					met = true
				} else if completed.Status == "Failed" || completed.Status == "Canceled" {
					return false, dep // dependency failed
				}
				break
			}
		}

		// Check persistence if not found in memory history
		if !met && o.Persistence != nil {
			job, err := o.Persistence.GetJob(dep)
			if err == nil {
				if job.Status == "Completed" {
					met = true
				} else if job.Status == "Failed" || job.Status == "Canceled" {
					return false, dep
				}
			}
		}

		if !met {
			return false, ""
		}
	}
	return true, ""
}

func (o *Orchestrator) processWorkItem(ctx context.Context, item WorkItem, retryCount int, logger *slog.Logger) error {
	return o.processWorkItemInternal(ctx, item, retryCount, false, logger)
}

func (o *Orchestrator) checkCircularDependencyLocked(newItem WorkItem) error {
	if len(newItem.DependsOn) == 0 {
		return nil
	}

	adj := make(map[string][]string)
	for id, job := range o.pendingJobs {
		adj[id] = job.WorkItem.DependsOn
	}
	adj[newItem.ID] = newItem.DependsOn

	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	var path []string

	var dfs func(node string) error
	dfs = func(node string) error {
		visited[node] = true
		recStack[node] = true
		path = append(path, node)

		for _, dep := range adj[node] {
			if !visited[dep] {
				if err := dfs(dep); err != nil {
					return err
				}
			} else if recStack[dep] {
				cycleStr := dep
				for i := len(path) - 1; i >= 0; i-- {
					cycleStr = path[i] + " -> " + cycleStr
					if path[i] == dep {
						break
					}
				}
				return fmt.Errorf("circular dependency detected: %s", cycleStr)
			}
		}

		recStack[node] = false
		path = path[:len(path)-1]
		return nil
	}

	return dfs(newItem.ID)
}

func (o *Orchestrator) processWorkItemInternal(ctx context.Context, item WorkItem, retryCount int, bypassApproval bool, logger *slog.Logger) error {
	o.mu.Lock()

	// If draining, do not accept any new submissions unless they are retries of existing jobs.
	// We allow retryCount > 0 because those are already internally scheduled auto-retries.
	if o.draining && retryCount == 0 && !bypassApproval {
		o.mu.Unlock()
		return ErrDraining
	}

	if _, exists := o.activeJobs[item.ID]; exists {
		o.mu.Unlock()
		return fmt.Errorf("job %s is already active", item.ID)
	}
	if jobInfo, exists := o.pendingJobs[item.ID]; exists {
		if jobInfo.Status == "Pending Approval" {
			o.mu.Unlock()
			return fmt.Errorf("job %s is already pending approval", item.ID)
		}
		o.mu.Unlock()
		return fmt.Errorf("job %s is already pending dependencies", item.ID)
	}

	if err := o.checkCircularDependencyLocked(item); err != nil {
		o.mu.Unlock()
		return err
	}

		if !item.RunAfter.IsZero() && time.Now().Before(item.RunAfter) {
			delay := item.RunAfter.Sub(time.Now())

			job := JobInfo{
				ID:         item.ID,
				Summary:    item.Summary,
				StartTime:  time.Now(),
				Status:     "Pending",
				WorkItem:   item,
				RetryCount: retryCount,
				Approved:   bypassApproval,
			}
			o.pendingJobs[item.ID] = job
			o.mu.Unlock()

			time.AfterFunc(delay, func() {
				o.evaluatePendingJobs(context.Background(), logger)
			})

			return nil
		}

	if o.RequireApproval && !bypassApproval {
		job := JobInfo{
			ID:         item.ID,
			Summary:    item.Summary,
			StartTime:  time.Now(),
			Status:     "Pending Approval",
			WorkItem:   item,
			RetryCount: retryCount,
			Approved:   false,
		}
		o.pendingJobs[item.ID] = job
		o.mu.Unlock()
		o.BroadcastEvent("job_pending_approval", job)
		if logger != nil {
			logger.Info("Job pending approval", "id", item.ID)
		}
		return nil
	}

	if len(item.DependsOn) > 0 {
		met, failedDep := o.checkDependenciesMetLocked(item.DependsOn)
		if !met {
			if failedDep != "" {
				// Dependency failed, immediately fail this job
				job := JobInfo{
					ID:        item.ID,
					Summary:   item.Summary,
					StartTime: time.Now(),
					EndTime:   time.Now(),
					Status:    "Failed",
					Error:     fmt.Sprintf("Dependency %s failed", failedDep),
					WorkItem:  item,
				}
				o.addToHistory(job, logger)
				o.mu.Unlock()
				o.BroadcastEvent("job_failed", job)
				if logger != nil {
					logger.Error("Job failed due to failed dependency", "id", item.ID, "dependency", failedDep)
				}
				return fmt.Errorf("dependency %s failed", failedDep)
			}
			// Valid dependencies but not yet completed
			job := JobInfo{
				ID:         item.ID,
				Summary:    item.Summary,
				StartTime:  time.Now(),
				Status:     "Pending",
				WorkItem:   item,
				RetryCount: retryCount,
			}
			o.pendingJobs[item.ID] = job
			o.mu.Unlock()
			o.BroadcastEvent("job_pending_deps", job)
			if logger != nil {
				logger.Info("Job pending dependencies", "id", item.ID, "depends_on", item.DependsOn)
			}
			return nil
		}
	}

	if o.MaxConcurrentJobs > 0 && o.activeSpawns >= o.MaxConcurrentJobs {
		o.mu.Unlock()
		return ErrAtCapacity
	}

	o.activeSpawns++
	o.totalSpawns++
	job := JobInfo{
		ID:         item.ID,
		Summary:    item.Summary,
		StartTime:  time.Now(),
		Status:     "Spawning",
		WorkItem:   item,
		RetryCount: retryCount,
		Approved:   bypassApproval,
	}
	o.activeJobs[item.ID] = job
	o.mu.Unlock()

	o.BroadcastEvent("job_spawning", job)

	o.wg.Add(1)
	go o.spawnWorker(ctx, item, logger)
	return nil
}

func (o *Orchestrator) spawnWorker(ctx context.Context, item WorkItem, logger *slog.Logger) {
	defer o.wg.Done()

	spawnCtx := ctx
	var cancel context.CancelFunc

	timeout := item.Timeout
	if timeout == 0 {
		timeout = o.JobTimeout
	}

	if timeout > 0 {
		spawnCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	var spawnErr error
	var threadState string

	if o.notifier != nil {
		ts, err := o.notifier.Notify(ctx, "on_start", fmt.Sprintf("Started job %s: %s", item.ID, item.Summary), "")
		if err != nil {
			logger.Warn("Failed to send start notification", "id", item.ID, "error", err)
		} else {
			threadState = ts
		}
	}

	defer func() {
		o.mu.Lock()
		var finalJob JobInfo
		var hasFinalJob bool

		// Move to history
		if job, ok := o.activeJobs[item.ID]; ok {
			job.ThreadState = threadState
			if spawnErr != nil {
				if o.MaxRetries > 0 && job.RetryCount < o.MaxRetries && spawnCtx.Err() != context.DeadlineExceeded && spawnCtx.Err() != context.Canceled {
					job.RetryCount++
					job.Status = "Retrying"
					job.Error = spawnErr.Error()
					job.RetryAfter = time.Now().Add(o.RetryDelay)
					o.pendingJobs[item.ID] = job
					finalJob = job
					hasFinalJob = true

					if logger != nil {
						logger.Info("Job failed, scheduling auto-retry", "id", item.ID, "attempt", job.RetryCount, "max", o.MaxRetries, "delay", o.RetryDelay)
					}

					// Trigger re-evaluation when delay expires
					delay := o.RetryDelay
					time.AfterFunc(delay, func() {
						o.evaluatePendingJobs(context.Background(), logger)
					})
				} else {
					job.EndTime = time.Now()
					job.Status = "Failed"
					job.Error = spawnErr.Error()
					o.addToHistory(job, logger)
					finalJob = job
					hasFinalJob = true
				}
			} else {
				job.EndTime = time.Now()
				job.Status = "Completed"
				o.addToHistory(job, logger)
				finalJob = job
				hasFinalJob = true
			}
		}

		o.activeSpawns--
		delete(o.activeJobs, item.ID)
		o.mu.Unlock()

		if hasFinalJob {
			if finalJob.Status == "Retrying" {
				o.BroadcastEvent("job_retrying", finalJob)
			} else if finalJob.Status == "Failed" {
				o.BroadcastEvent("job_failed", finalJob)
			} else if finalJob.Status == "Completed" {
				o.BroadcastEvent("job_completed", finalJob)
			}
		}

		if o.notifier != nil {
			if spawnErr != nil {
				_, _ = o.notifier.Notify(ctx, "on_failure", fmt.Sprintf("Job %s failed: %v", item.ID, spawnErr), threadState)
			} else {
				_, _ = o.notifier.Notify(ctx, "on_success", fmt.Sprintf("Job %s completed successfully", item.ID), threadState)
			}
		}

		if o.LogDir != "" {
			safeID := filepath.Base(item.ID)
			if safeID != "." && safeID != ".." && safeID != "/" && safeID != "\\" && !strings.Contains(safeID, "/") && !strings.Contains(safeID, "\\") {
				// Don't impose an arbitrary timeout, let it stream until EOF
				logsReader, err := o.Spawner.GetLogs(context.Background(), item.ID)
				if err == nil && logsReader != nil {
					if err := os.MkdirAll(o.LogDir, 0755); err == nil {
						logPath := filepath.Join(o.LogDir, fmt.Sprintf("%s.log.gz", safeID))
						tmpPath := logPath + ".tmp"
						f, err := os.Create(tmpPath)
						if err == nil {
							gzWriter := gzip.NewWriter(f)
							_, ioErr := io.Copy(gzWriter, logsReader)
							gzWriter.Close()
							f.Close()
							if ioErr != nil && logger != nil {
								logger.Warn("Failed to copy logs to persistent storage", "id", item.ID, "error", ioErr)
							} else {
								// Only move to final path after writing completely
								os.Rename(tmpPath, logPath)
							}
						} else if logger != nil {
							logger.Warn("Failed to create temp log file", "path", tmpPath, "error", err)
						}
					} else if logger != nil {
						logger.Warn("Failed to create log directory", "path", o.LogDir, "error", err)
					}
					logsReader.Close()
				} else if logger != nil {
					logger.Warn("Failed to retrieve logs from spawner for persistence", "id", item.ID, "error", err)
				}
			}
		}

		o.evaluatePendingJobs(ctx, logger)
	}()

	if logger != nil {
		logger.Info("Spawning agent for item", "id", item.ID)
	}

	if err := o.Spawner.Spawn(spawnCtx, item); err != nil {
		spawnErr = err
		if spawnCtx.Err() == context.DeadlineExceeded {
			if logger != nil {
				logger.Error("Job timeout exceeded", "id", item.ID, "error", err)
			}
			_ = o.Poller.UpdateStatus(ctx, item, "Failed", fmt.Sprintf("Job timeout exceeded: %v", err))
		} else {
			if logger != nil {
				logger.Error("Failed to spawn agent", "id", item.ID, "error", err)
			}
			// Update status to Failed
			_ = o.Poller.UpdateStatus(ctx, item, "Failed", fmt.Sprintf("Failed to spawn agent: %v", err))
		}
	} else {
		// Success? K8s Jobs are fire-and-forget from Spawner perspective usually,
		// but status updates might happen asynchronously.
		// For now, Spawn() implies "Started".
		if logger != nil {
			logger.Info("Agent spawned successfully", "id", item.ID)
		}
	}
}

func (o *Orchestrator) evaluatePendingJobs(ctx context.Context, logger *slog.Logger) {
	o.mu.Lock()
	type pendingJob struct {
		item       WorkItem
		retryCount int
		approved   bool
	}
	var toProcess []pendingJob
	for id, jobInfo := range o.pendingJobs {
		if !jobInfo.RetryAfter.IsZero() && time.Now().Before(jobInfo.RetryAfter) {
			continue
		}

		if !jobInfo.WorkItem.RunAfter.IsZero() && time.Now().Before(jobInfo.WorkItem.RunAfter) {
			continue
		}

		// If require approval is true, and it hasn't been approved, skip it.
		// If it has been approved, we process it normally (check dependencies, spawn)
		if o.RequireApproval && !jobInfo.Approved {
			continue
		}

		item := jobInfo.WorkItem
		met, failedDep := o.checkDependenciesMetLocked(item.DependsOn)
		if met {
			toProcess = append(toProcess, pendingJob{item: item, retryCount: jobInfo.RetryCount, approved: jobInfo.Approved})
			delete(o.pendingJobs, id)
		} else if failedDep != "" {
			// Dependency failed, fail this job too
			delete(o.pendingJobs, id)
			job := JobInfo{
				ID:        item.ID,
				Summary:   item.Summary,
				StartTime: time.Now(),
				EndTime:   time.Now(),
				Status:    "Failed",
				Error:     fmt.Sprintf("Dependency %s failed", failedDep),
				WorkItem:  item,
			}
			o.addToHistory(job, logger)
			// Ensure we broadcast after dropping the lock to avoid deadlocks
			defer o.BroadcastEvent("job_failed", job)
		}
	}
	o.mu.Unlock()

	// Sort pending jobs by Priority (descending) and ID (ascending) to ensure stable processing order
	sort.SliceStable(toProcess, func(i, j int) bool {
		if toProcess[i].item.Priority != toProcess[j].item.Priority {
			return toProcess[i].item.Priority > toProcess[j].item.Priority
		}
		return toProcess[i].item.ID < toProcess[j].item.ID
	})

	for _, pJob := range toProcess {
		item := pJob.item
		if err := o.processWorkItemInternal(ctx, item, pJob.retryCount, pJob.approved, logger); err != nil {
			if logger != nil {
				logger.Error("Failed to start pending job", "id", item.ID, "error", err)
			}
			if err == ErrAtCapacity {
				// Put it back in pendingJobs
				o.mu.Lock()
				o.pendingJobs[item.ID] = JobInfo{
					ID:         item.ID,
					Summary:    item.Summary,
					StartTime:  time.Now(),
					Status:     "Pending",
					WorkItem:   item,
					RetryCount: pJob.retryCount,
					Approved:   pJob.approved,
				}
				o.mu.Unlock()
			}
		}
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

// PurgeJob removes a specific job from history (both in-memory and persistent storage).
// It returns an error if the job is currently active or pending.
func (o *Orchestrator) PurgeJob(id string, logger *slog.Logger) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if _, exists := o.activeJobs[id]; exists {
		return fmt.Errorf("job %s is active, cannot purge", id)
	}
	if _, exists := o.pendingJobs[id]; exists {
		return fmt.Errorf("job %s is pending, cannot purge", id)
	}

	// Remove from in-memory history
	foundInMemory := false
	for i, job := range o.completedJobs {
		if job.ID == id {
			foundInMemory = true
			// Remove element
			o.completedJobs = append(o.completedJobs[:i], o.completedJobs[i+1:]...)
			break
		}
	}

	// Remove from persistence
	dbPurged := false
	if o.Persistence != nil {
		err := o.Persistence.PurgeJob(id)
		if err == nil {
			dbPurged = true
		} else if err.Error() != fmt.Sprintf("job %s not found", id) {
			return fmt.Errorf("failed to purge job from persistence: %w", err)
		}
	}

	if !foundInMemory && !dbPurged {
		return fmt.Errorf("job %s not found in history", id)
	}

	if logger != nil {
		logger.Info("Job purged from history", "id", id)
	}

	o.BroadcastEvent("job_purged", map[string]string{"id": id})

	return nil
}

// ClearHistory clears the in-memory history and the persistent history.
func (o *Orchestrator) ClearHistory(logger *slog.Logger) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	count := len(o.completedJobs)
	o.completedJobs = nil

	if o.Persistence != nil {
		dbCount, err := o.Persistence.ClearHistory()
		if err != nil {
			return 0, fmt.Errorf("failed to clear persistent history: %w", err)
		}
		// If db has more, trust the db count
		if dbCount > count {
			count = dbCount
		}
	}

	if logger != nil {
		logger.Info("History cleared", "count", count)
	}

	return count, nil
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
			if err := o.processWorkItem(ctx, item, 0, logger); err != nil {
				if err == ErrAtCapacity {
					logger.Info("Orchestrator at max capacity, deferring remaining items", "max", o.MaxConcurrentJobs)
					break // Stop processing this batch
				}
				// Log duplication as info, but real errors as errors
				if err.Error() == fmt.Sprintf("job %s is already active", item.ID) {
					logger.Info("Job already active, skipping", "id", item.ID)
				} else {
					logger.Error("Failed to process work item", "id", item.ID, "error", err)
				}
			}
		}

		// Also evaluate pending jobs in case capacity freed up but no jobs completed recently
		o.evaluatePendingJobs(ctx, logger)
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
			draining := o.draining
			o.mu.RUnlock()

			if !paused && !draining {
				poll()
			} else if draining {
				logger.Debug("Orchestrator is draining, skipping poll")
			} else {
				logger.Debug("Orchestrator is paused, skipping poll")
			}
		case <-o.forcePollCh:
			o.mu.RLock()
			paused := o.paused
			draining := o.draining
			o.mu.RUnlock()

			if !paused && !draining {
				logger.Info("Force poll triggered")
				poll()
			} else if draining {
				logger.Debug("Orchestrator is draining, skipping force poll")
			} else {
				logger.Debug("Orchestrator is paused, skipping force poll")
			}
		}
	}
}
