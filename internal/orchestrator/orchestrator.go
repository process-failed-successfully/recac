package orchestrator

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
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
	delayTimers       map[string]*time.Timer
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

	CircuitBreakerMaxFailures int
	ConsecutiveSpawnFailures  int
	CircuitBroken             bool

	eventChans map[chan []byte]struct{}
	eventMu    sync.RWMutex
}

var ErrAtCapacity = fmt.Errorf("orchestrator is at max capacity")
var ErrDraining = fmt.Errorf("orchestrator is draining and cannot accept new jobs")

type JobInfo struct {
	ID            string             `json:"id"`
	Summary       string             `json:"summary"`
	StartTime     time.Time          `json:"start_time"`
	EndTime       time.Time          `json:"end_time,omitempty"`
	Status        string             `json:"status"`
	Error         string             `json:"error,omitempty"`
	WorkItem      WorkItem           `json:"work_item"`
	ThreadState   string             `json:"thread_state,omitempty"`
	RetryCount    int                `json:"retry_count,omitempty"`
	RetryAfter    time.Time          `json:"retry_after,omitempty"`
	Approved      bool               `json:"approved,omitempty"`
	Outputs       map[string]string  `json:"outputs,omitempty"`
	Metrics       map[string]float64 `json:"metrics,omitempty"`
	Progress      *int               `json:"progress,omitempty"`
	StatusMessage *string            `json:"status_message,omitempty"`
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
	CircuitBroken     bool      `json:"circuit_broken"`
	MaxConcurrentJobs int       `json:"max_concurrent_jobs"`
}

type Analytics struct {
	TotalJobs       int                `json:"total_jobs"`
	SuccessfulJobs  int                `json:"successful_jobs"`
	FailedJobs      int                `json:"failed_jobs"`
	CanceledJobs    int                `json:"canceled_jobs"`
	SkippedJobs     int                `json:"skipped_jobs"`
	SuccessRate     float64            `json:"success_rate"`
	AverageDuration time.Duration      `json:"average_duration"`
	TotalMetrics    map[string]float64 `json:"total_metrics,omitempty"`
}


// recordSpawnFailure increments the consecutive failure counter and checks the circuit breaker.
func (o *Orchestrator) recordSpawnFailure(logger *slog.Logger) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.ConsecutiveSpawnFailures++

	if o.CircuitBreakerMaxFailures > 0 && o.ConsecutiveSpawnFailures >= o.CircuitBreakerMaxFailures && !o.CircuitBroken {
		o.CircuitBroken = true
		o.paused = true
		if logger != nil {
			logger.Error("Circuit breaker tripped: too many consecutive spawn failures. Orchestrator paused.", "failures", o.ConsecutiveSpawnFailures)
		}
		// We use a goroutine so BroadcastEvent doesn't deadlock if there's any lock contention
		go o.BroadcastEvent("circuit_breaker_tripped", map[string]interface{}{
			"failures": o.ConsecutiveSpawnFailures,
		})
	}
}

// recordSpawnSuccess resets the consecutive spawn failure counter.
func (o *Orchestrator) recordSpawnSuccess() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.ConsecutiveSpawnFailures > 0 {
		o.ConsecutiveSpawnFailures = 0
	}
}

func New(poller Poller, spawner Spawner, pollInterval time.Duration) *Orchestrator {
	return &Orchestrator{
		Poller:       poller,
		Spawner:      spawner,
		PollInterval: pollInterval,
		activeJobs:   make(map[string]JobInfo),
		pendingJobs:  make(map[string]JobInfo),
		delayTimers:  make(map[string]*time.Timer),
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
func (o *Orchestrator) getJobLocked(id string) (JobInfo, bool) {
	job, exists := o.activeJobs[id]
	if exists {
		return job, true
	}

	job, exists = o.pendingJobs[id]
	if exists {
		return job, true
	}

	// Check completed jobs (reverse order for most recent)
	for i := len(o.completedJobs) - 1; i >= 0; i-- {
		if o.completedJobs[i].ID == id {
			return o.completedJobs[i], true
		}
	}

	// Check persistence if available
	if o.Persistence != nil {
		job, err := o.Persistence.GetJob(id)
		if err == nil {
			return *job, true
		}
	}

	return JobInfo{}, false
}

func (o *Orchestrator) GetJob(id string) (JobInfo, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	job, found := o.getJobLocked(id)
	if !found {
		return JobInfo{}, fmt.Errorf("job %s not found", id)
	}
	return job, nil
}

// GetJobBlockers returns a list of jobs that are blocking the given job from running.
func (o *Orchestrator) GetJobBlockers(id string) ([]JobInfo, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	targetJob, found := o.getJobLocked(id)
	if !found {
		return nil, fmt.Errorf("job %s not found", id)
	}

	if len(targetJob.WorkItem.DependsOn) == 0 {
		return nil, nil // No dependencies, no blockers
	}

	var blockers []JobInfo
	for _, depID := range targetJob.WorkItem.DependsOn {
		depJob, depFound := o.getJobLocked(depID)
		if !depFound {
			blockers = append(blockers, JobInfo{
				ID:      depID,
				Status:  "Missing",
				Summary: "Dependency not found in active, pending, or history",
			})
		} else {
			status := strings.ToLower(depJob.Status)
			if status == "pending" || status == "pending approval" || status == "spawning" || status == "running" || status == "active" || status == "retrying" {
				blockers = append(blockers, depJob)
			} else if status == "failed" || status == "canceled" {
				cond := strings.ToLower(strings.TrimSpace(targetJob.WorkItem.RunCondition))
				if cond != "always" && cond != "on_failure" {
					blockers = append(blockers, depJob)
				}
			}
		}
	}

	return blockers, nil
}
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
	if t, ok := o.delayTimers[jobID]; ok {
		t.Stop()
		delete(o.delayTimers, jobID)
	}
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

// DeletePendingJob removes a pending job without keeping history.
func (o *Orchestrator) DeletePendingJob(ctx context.Context, jobID string, logger *slog.Logger) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	job, exists := o.pendingJobs[jobID]
	if !exists {
		// Check if active or completed to return a more specific error
		if _, active := o.activeJobs[jobID]; active {
			return fmt.Errorf("job %s is already active and cannot be deleted", jobID)
		}
		for _, completed := range o.completedJobs {
			if completed.ID == jobID {
				return fmt.Errorf("job %s is already completed", jobID)
			}
		}
		return fmt.Errorf("job %s not found in pending queue", jobID)
	}

	if t, ok := o.delayTimers[jobID]; ok {
		t.Stop()
		delete(o.delayTimers, jobID)
	}
	delete(o.pendingJobs, jobID)
	o.BroadcastEvent("job_deleted", job)

	if logger != nil {
		logger.Info("Job deleted from pending queue", "jobID", jobID)
	}
	return nil
}

// DeletePendingJobsByTag removes pending jobs matching the given tag.
func (o *Orchestrator) DeletePendingJobsByTag(ctx context.Context, tag string, logger *slog.Logger) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	count := 0
	lowerTag := strings.ToLower(tag)

	for id, job := range o.pendingJobs {
		for _, t := range job.WorkItem.Tags {
			if strings.ToLower(t) == lowerTag {
				if t_timer, ok := o.delayTimers[id]; ok {
					t_timer.Stop()
					delete(o.delayTimers, id)
				}
				delete(o.pendingJobs, id)
				o.BroadcastEvent("job_deleted", job)
				count++
				break
			}
		}
	}

	if logger != nil && count > 0 {
		logger.Info("Deleted jobs from pending queue by tag", "tag", tag, "count", count)
	}
	return count, nil
}

// DeletePendingJobsByMatch removes pending jobs matching the regex.
func (o *Orchestrator) DeletePendingJobsByMatch(ctx context.Context, match string, logger *slog.Logger) (int, error) {
	matcher, err := regexp.Compile("(?i)" + match)
	if err != nil {
		return 0, fmt.Errorf("invalid match regex: %w", err)
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	count := 0

	for id, job := range o.pendingJobs {
		if matcher.MatchString(job.Summary) || matcher.MatchString(job.Error) {
			if t_timer, ok := o.delayTimers[id]; ok {
				t_timer.Stop()
				delete(o.delayTimers, id)
			}
			delete(o.pendingJobs, id)
			o.BroadcastEvent("job_deleted", job)
			count++
		}
	}

	if logger != nil && count > 0 {
		logger.Info("Deleted jobs from pending queue by match", "match", match, "count", count)
	}
	return count, nil
}

// ClearPendingJobs cancels all jobs currently waiting for dependencies.
func (o *Orchestrator) ClearPendingJobs(ctx context.Context, logger *slog.Logger) int {
	o.mu.Lock()
	defer o.mu.Unlock()

	count := 0
	for id, job := range o.pendingJobs {
		if t, ok := o.delayTimers[id]; ok {
			t.Stop()
			delete(o.delayTimers, id)
		}
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

// CancelJobsByStatus cancels all running and pending jobs that match the specified status (case-insensitive).
func (o *Orchestrator) CancelJobsByStatus(ctx context.Context, status string, logger *slog.Logger) (int, error) {
	o.mu.Lock()
	var jobIDs []string
	lowerStatus := strings.ToLower(status)

	for id, job := range o.activeJobs {
		if strings.ToLower(job.Status) == lowerStatus {
			jobIDs = append(jobIDs, id)
		}
	}
	for id, job := range o.pendingJobs {
		if strings.ToLower(job.Status) == lowerStatus {
			jobIDs = append(jobIDs, id)
		}
	}
	o.mu.Unlock()

	if logger != nil && len(jobIDs) > 0 {
		logger.Info("Canceling jobs by status", "status", status, "count", len(jobIDs))
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

// CancelJobsByMatch cancels all running and pending jobs where the summary or error matches the given regex.
func (o *Orchestrator) CancelJobsByMatch(ctx context.Context, match string, logger *slog.Logger) (int, error) {
	matcher, err := regexp.Compile("(?i)" + match)
	if err != nil {
		return 0, fmt.Errorf("invalid match regex: %w", err)
	}

	o.mu.Lock()
	var jobIDs []string

	for id, job := range o.activeJobs {
		if matcher.MatchString(job.Summary) || matcher.MatchString(job.Error) {
			jobIDs = append(jobIDs, id)
		}
	}
	for id, job := range o.pendingJobs {
		if matcher.MatchString(job.Summary) || matcher.MatchString(job.Error) {
			jobIDs = append(jobIDs, id)
		}
	}
	o.mu.Unlock()

	if logger != nil && len(jobIDs) > 0 {
		logger.Info("Canceling jobs by match", "match", match, "count", len(jobIDs))
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

// ForceCompleteJob forcefully marks an active, pending, or failed job as Completed, bypassing execution.
func (o *Orchestrator) ForceCompleteJob(ctx context.Context, jobID string, logger *slog.Logger) error {
	o.mu.Lock()

	var jobToComplete JobInfo
	var found bool
	var isActive bool

	// 1. Check pending jobs
	if job, exists := o.pendingJobs[jobID]; exists {
		jobToComplete = job
		found = true
		delete(o.pendingJobs, jobID)
		if t, ok := o.delayTimers[jobID]; ok {
			t.Stop()
			delete(o.delayTimers, jobID)
		}
	}

	// 2. Check active jobs
	if !found {
		if job, exists := o.activeJobs[jobID]; exists {
			jobToComplete = job
			found = true
			isActive = true
			delete(o.activeJobs, jobID)
			o.activeSpawns--
		}
	}

	// 3. Check history
	if !found {
		for i := len(o.completedJobs) - 1; i >= 0; i-- {
			if o.completedJobs[i].ID == jobID {
				if o.completedJobs[i].Status == "Failed" || o.completedJobs[i].Status == "Canceled" {
					jobToComplete = o.completedJobs[i]
					found = true
					// Remove from completed list so we can re-add it as completed
					o.completedJobs = append(o.completedJobs[:i], o.completedJobs[i+1:]...)
				} else if o.completedJobs[i].Status == "Completed" {
					o.mu.Unlock()
					return fmt.Errorf("job %s is already completed", jobID)
				} else if o.completedJobs[i].Status == "Skipped" {
					o.mu.Unlock()
					return fmt.Errorf("job %s is already skipped", jobID)
				}
				break
			}
		}
	}

	if !found {
		o.mu.Unlock()
		return fmt.Errorf("job %s not found or not in a completable state", jobID)
	}

	jobToComplete.Status = "Completed"
	jobToComplete.EndTime = time.Now()
	jobToComplete.Error = ""
	o.addToHistory(jobToComplete, logger)
	o.mu.Unlock()

	if jobToComplete.WorkItem.WebhookURL != "" {
		go o.fireJobWebhook(jobToComplete, logger)
	}

	// 4. Cancel active spawner if it was active
	if isActive {
		// Attempt to cancel underlying resource, ignore error as we force complete
		_ = o.Spawner.Cancel(ctx, jobID)
	}

	o.BroadcastEvent("job_completed", jobToComplete)

	if logger != nil {
		logger.Info("Job force completed", "jobID", jobID)
	}

	// 5. Trigger downstream jobs
	o.evaluatePendingJobs(ctx, logger)

	return nil
}

// ForceCompleteJobsByTag force completes jobs matching the given tag.
func (o *Orchestrator) ForceCompleteJobsByTag(ctx context.Context, tag string, logger *slog.Logger) (int, error) {
	o.mu.RLock()
	var jobIDs []string
	lowerTag := strings.ToLower(tag)

	// We gather IDs to avoid locking while calling ForceCompleteJob
	for id, job := range o.pendingJobs {
		for _, t := range job.WorkItem.Tags {
			if strings.ToLower(t) == lowerTag {
				jobIDs = append(jobIDs, id)
				break
			}
		}
	}
	for id, job := range o.activeJobs {
		for _, t := range job.WorkItem.Tags {
			if strings.ToLower(t) == lowerTag {
				jobIDs = append(jobIDs, id)
				break
			}
		}
	}
	for _, job := range o.completedJobs {
		if job.Status == "Failed" || job.Status == "Canceled" {
			for _, t := range job.WorkItem.Tags {
				if strings.ToLower(t) == lowerTag {
					jobIDs = append(jobIDs, job.ID)
					break
				}
			}
		}
	}
	o.mu.RUnlock()

	count := 0
	for _, id := range jobIDs {
		if err := o.ForceCompleteJob(ctx, id, logger); err == nil {
			count++
		}
	}

	if logger != nil && count > 0 {
		logger.Info("Force completed jobs by tag", "tag", tag, "count", count)
	}

	return count, nil
}

// ForceCompleteJobsByMatch force completes jobs matching the given regex.
func (o *Orchestrator) ForceCompleteJobsByMatch(ctx context.Context, match string, logger *slog.Logger) (int, error) {
	matcher, err := regexp.Compile("(?i)" + match)
	if err != nil {
		return 0, fmt.Errorf("invalid match regex: %w", err)
	}

	o.mu.RLock()
	var jobIDs []string

	for id, job := range o.pendingJobs {
		if matcher.MatchString(job.Summary) || matcher.MatchString(job.Error) {
			jobIDs = append(jobIDs, id)
		}
	}
	for id, job := range o.activeJobs {
		if matcher.MatchString(job.Summary) || matcher.MatchString(job.Error) {
			jobIDs = append(jobIDs, id)
		}
	}
	for _, job := range o.completedJobs {
		if job.Status == "Failed" || job.Status == "Canceled" {
			if matcher.MatchString(job.Summary) || matcher.MatchString(job.Error) {
				jobIDs = append(jobIDs, job.ID)
			}
		}
	}
	o.mu.RUnlock()

	count := 0
	for _, id := range jobIDs {
		if err := o.ForceCompleteJob(ctx, id, logger); err == nil {
			count++
		}
	}

	if logger != nil && count > 0 {
		logger.Info("Force completed jobs by match", "match", match, "count", count)
	}

	return count, nil
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

	stats.TotalMetrics = make(map[string]float64)

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
		} else if job.Status == "Skipped" {
			stats.SkippedJobs++
		}

		if job.Metrics != nil {
			for k, v := range job.Metrics {
				stats.TotalMetrics[k] += v
			}
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

// SkipJob forcefully marks a pending job as Skipped, bypassing execution.
func (o *Orchestrator) SkipJob(ctx context.Context, jobID string, logger *slog.Logger) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if job, exists := o.pendingJobs[jobID]; exists {
		delete(o.pendingJobs, jobID)
		job.Status = "Skipped"
		job.EndTime = time.Now()
		job.Error = "Skipped by user"
		o.addToHistory(job, logger)
		o.BroadcastEvent("job_skipped", job)
		if logger != nil {
			logger.Info("Job skipped", "jobID", jobID)
		}
		return nil
	}

	return fmt.Errorf("job %s not found in pending queue", jobID)
}

// SkipJobsByTag marks all pending jobs with the specified tag as Skipped.
func (o *Orchestrator) SkipJobsByTag(ctx context.Context, tag string, logger *slog.Logger) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	count := 0
	lowerTag := strings.ToLower(tag)

	for id, job := range o.pendingJobs {
		for _, t := range job.WorkItem.Tags {
			if strings.ToLower(t) == lowerTag {
				delete(o.pendingJobs, id)
				job.Status = "Skipped"
				job.EndTime = time.Now()
				job.Error = "Skipped by tag: " + tag
				o.addToHistory(job, logger)
				o.BroadcastEvent("job_skipped", job)
				count++
				break
			}
		}
	}
	return count, nil
}

// SkipJobsByMatch marks all pending jobs matching the regex as Skipped.
func (o *Orchestrator) SkipJobsByMatch(ctx context.Context, match string, logger *slog.Logger) (int, error) {
	matcher, err := regexp.Compile("(?i)" + match)
	if err != nil {
		return 0, fmt.Errorf("invalid match regex: %w", err)
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	count := 0

	for id, job := range o.pendingJobs {
		if matcher.MatchString(job.Summary) || matcher.MatchString(job.Error) {
			delete(o.pendingJobs, id)
			job.Status = "Skipped"
			job.EndTime = time.Now()
			job.Error = "Skipped by match"
			o.addToHistory(job, logger)
			o.BroadcastEvent("job_skipped", job)
			count++
		}
	}
	return count, nil
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
		CircuitBroken:     o.CircuitBroken,
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

// Resume resumes the orchestrator's polling loop and resets the circuit breaker.
func (o *Orchestrator) Resume() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.paused = false
	o.CircuitBroken = false
	o.ConsecutiveSpawnFailures = 0
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

// ApproveJobsByTag approves pending jobs that match the given tag and are pending approval.
func (o *Orchestrator) ApproveJobsByTag(ctx context.Context, tag string, logger *slog.Logger) (int, error) {
	o.mu.Lock()
	count := 0
	lowerTag := strings.ToLower(tag)

	for id, job := range o.pendingJobs {
		if job.Status != "Pending Approval" {
			continue
		}

		hasTag := false
		for _, t := range job.WorkItem.Tags {
			if strings.ToLower(t) == lowerTag {
				hasTag = true
				break
			}
		}

		if hasTag {
			job.Status = "Pending"
			job.Approved = true
			o.pendingJobs[id] = job
			count++
			o.BroadcastEvent("job_approved", job)
			if logger != nil {
				logger.Info("Job approved by tag", "id", id, "tag", tag)
			}
		}
	}
	o.mu.Unlock()

	if logger != nil && count > 0 {
		logger.Info("Approved jobs by tag", "tag", tag, "count", count)
	}

	if count > 0 {
		o.evaluatePendingJobs(ctx, logger)
	}

	return count, nil
}

// ApproveJobsByMatch approves pending jobs whose ID matches the given regular expression and are pending approval.
func (o *Orchestrator) ApproveJobsByMatch(ctx context.Context, match string, logger *slog.Logger) (int, error) {
	matcher, err := regexp.Compile("(?i)" + match)
	if err != nil {
		return 0, fmt.Errorf("invalid regex pattern: %w", err)
	}

	o.mu.Lock()
	count := 0

	for id, job := range o.pendingJobs {
		if job.Status != "Pending Approval" {
			continue
		}

		if matcher.MatchString(id) {
			job.Status = "Pending"
			job.Approved = true
			o.pendingJobs[id] = job
			count++
			o.BroadcastEvent("job_approved", job)
			if logger != nil {
				logger.Info("Job approved by match", "id", id, "match", match)
			}
		}
	}
	o.mu.Unlock()

	if logger != nil && count > 0 {
		logger.Info("Approved jobs by match", "match", match, "count", count)
	}

	if count > 0 {
		o.evaluatePendingJobs(ctx, logger)
	}

	return count, nil
}

// UpdateJobDependencies updates the dependencies of a job in the pending queue.
func (o *Orchestrator) UpdateJobDependencies(ctx context.Context, jobID string, dependsOn []string, logger *slog.Logger) error {
	o.mu.Lock()
	job, exists := o.pendingJobs[jobID]
	if !exists {
		o.mu.Unlock()
		// Check if active or completed to return a more specific error
		o.mu.RLock()
		if _, active := o.activeJobs[jobID]; active {
			o.mu.RUnlock()
			return fmt.Errorf("job %s is already active and cannot have dependencies updated", jobID)
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

	// Create a backup of old dependencies to revert if cycle detected
	oldDeps := job.WorkItem.DependsOn

	// Make a copy of the new dependencies to avoid mutating original slice accidentally
	newDeps := make([]string, len(dependsOn))
	copy(newDeps, dependsOn)
	job.WorkItem.DependsOn = newDeps

	// Update the map so checkCircularDependencyLocked can see the new graph
	o.pendingJobs[jobID] = job

	// Temporarily create a dummy WorkItem for cycle detection
	dummyItem := job.WorkItem

	if err := o.checkCircularDependencyLocked(dummyItem); err != nil {
		// Revert
		job.WorkItem.DependsOn = oldDeps
		o.pendingJobs[jobID] = job
		o.mu.Unlock()
		return err
	}

	if o.Persistence != nil {
		o.Persistence.SaveJob(job)
	}

	o.mu.Unlock()
	o.BroadcastEvent("job_dependencies_updated", job)

	if logger != nil {
		logger.Info("Updated job dependencies", "jobID", jobID, "dependsOn", newDeps)
	}

	o.evaluatePendingJobs(ctx, logger)
	return nil
}

// UpdateJobsDependenciesByTag updates the dependencies of pending jobs that match the given tag.
func (o *Orchestrator) UpdateJobsDependenciesByTag(ctx context.Context, tag string, dependsOn []string, logger *slog.Logger) (int, error) {
	o.mu.Lock()
	var jobIDs []string
	lowerTag := strings.ToLower(tag)

	for id, job := range o.pendingJobs {
		for _, t := range job.WorkItem.Tags {
			if strings.ToLower(t) == lowerTag {
				jobIDs = append(jobIDs, id)
				break
			}
		}
	}
	o.mu.Unlock()

	count := 0
	for _, id := range jobIDs {
		if err := o.UpdateJobDependencies(ctx, id, dependsOn, logger); err == nil {
			count++
		}
	}

	return count, nil
}

// UpdateJobsDependenciesByMatch updates the dependencies of pending jobs that match the given regex.
func (o *Orchestrator) UpdateJobsDependenciesByMatch(ctx context.Context, match string, dependsOn []string, logger *slog.Logger) (int, error) {
	matcher, err := regexp.Compile("(?i)" + match)
	if err != nil {
		return 0, fmt.Errorf("invalid match regex: %w", err)
	}

	o.mu.Lock()
	var jobIDs []string
	for id, job := range o.pendingJobs {
		if matcher.MatchString(job.Summary) || matcher.MatchString(job.Error) {
			jobIDs = append(jobIDs, id)
		}
	}
	o.mu.Unlock()

	count := 0
	for _, id := range jobIDs {
		if err := o.UpdateJobDependencies(ctx, id, dependsOn, logger); err == nil {
			count++
		}
	}

	return count, nil
}

// UpdateJobEnv updates the environment variables of a job in the pending queue.
func (o *Orchestrator) UpdateJobEnv(ctx context.Context, jobID string, envVars map[string]string, logger *slog.Logger) error {
	o.mu.Lock()
	job, exists := o.pendingJobs[jobID]
	if !exists {
		o.mu.Unlock()
		// Check if active or completed to return a more specific error
		o.mu.RLock()
		if _, active := o.activeJobs[jobID]; active {
			o.mu.RUnlock()
			return fmt.Errorf("job %s is already active and cannot have env updated", jobID)
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

	job.WorkItem.EnvVars = envVars
	o.pendingJobs[jobID] = job

	if o.Persistence != nil {
		o.Persistence.SaveJob(job)
	}

	o.mu.Unlock()
	o.BroadcastEvent("job_env_updated", job)

	if logger != nil {
		logger.Info("Updated job environment variables", "jobID", jobID, "envVars", envVars)
	}

	return nil
}

// UpdateJobsEnvByTag updates the environment variables of pending jobs that match the given tag.
func (o *Orchestrator) UpdateJobsEnvByTag(ctx context.Context, tag string, envVars map[string]string, logger *slog.Logger) (int, error) {
	o.mu.Lock()
	var jobIDs []string
	lowerTag := strings.ToLower(tag)

	for id, job := range o.pendingJobs {
		for _, t := range job.WorkItem.Tags {
			if strings.ToLower(t) == lowerTag {
				jobIDs = append(jobIDs, id)
				break
			}
		}
	}
	o.mu.Unlock()

	count := 0
	for _, id := range jobIDs {
		// UpdateJobEnv correctly acquires and releases the lock for each job.
		// It also checks that the job is still pending.
		if err := o.UpdateJobEnv(ctx, id, envVars, logger); err == nil {
			count++
		}
	}

	return count, nil
}

// UpdateJobsEnvByMatch updates the environment variables of pending jobs that match the given regex.
func (o *Orchestrator) UpdateJobsEnvByMatch(ctx context.Context, match string, envVars map[string]string, logger *slog.Logger) (int, error) {
	matcher, err := regexp.Compile("(?i)" + match)
	if err != nil {
		return 0, fmt.Errorf("invalid match regex: %w", err)
	}

	o.mu.Lock()
	var jobIDs []string
	for id, job := range o.pendingJobs {
		if matcher.MatchString(job.Summary) || matcher.MatchString(job.Error) {
			jobIDs = append(jobIDs, id)
		}
	}
	o.mu.Unlock()

	count := 0
	for _, id := range jobIDs {
		if err := o.UpdateJobEnv(ctx, id, envVars, logger); err == nil {
			count++
		}
	}

	return count, nil
}

// UpdateJobTags updates the tags of a job in the pending queue.
func (o *Orchestrator) UpdateJobTags(ctx context.Context, jobID string, tags []string, logger *slog.Logger) error {
	o.mu.Lock()
	job, exists := o.pendingJobs[jobID]
	if !exists {
		o.mu.Unlock()
		// Check if active or completed to return a more specific error
		o.mu.RLock()
		if _, active := o.activeJobs[jobID]; active {
			o.mu.RUnlock()
			return fmt.Errorf("job %s is already active and cannot have tags updated", jobID)
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

	job.WorkItem.Tags = tags
	o.pendingJobs[jobID] = job
	if o.Persistence != nil {
		o.Persistence.SaveJob(job)
	}
	o.mu.Unlock()
	o.BroadcastEvent("job_tags_updated", job)

	if logger != nil {
		logger.Info("Updated job tags", "jobID", jobID, "tags", tags)
	}

	return nil
}

// UpdateJobsTagsByTag updates the tags of pending jobs that match the given tag.
func (o *Orchestrator) UpdateJobsTagsByTag(ctx context.Context, tag string, tags []string, logger *slog.Logger) (int, error) {
	o.mu.Lock()
	var jobIDs []string
	lowerTag := strings.ToLower(tag)

	for id, job := range o.pendingJobs {
		for _, t := range job.WorkItem.Tags {
			if strings.ToLower(t) == lowerTag {
				jobIDs = append(jobIDs, id)
				break
			}
		}
	}
	o.mu.Unlock()

	count := 0
	for _, id := range jobIDs {
		if err := o.UpdateJobTags(ctx, id, tags, logger); err == nil {
			count++
		}
	}

	return count, nil
}

// UpdateJobsTagsByMatch updates the tags of pending jobs that match the given regex.
func (o *Orchestrator) UpdateJobsTagsByMatch(ctx context.Context, match string, tags []string, logger *slog.Logger) (int, error) {
	matcher, err := regexp.Compile("(?i)" + match)
	if err != nil {
		return 0, fmt.Errorf("invalid match regex: %w", err)
	}

	o.mu.Lock()
	var jobIDs []string
	for id, job := range o.pendingJobs {
		if matcher.MatchString(job.Summary) || matcher.MatchString(job.Error) {
			jobIDs = append(jobIDs, id)
		}
	}
	o.mu.Unlock()

	count := 0
	for _, id := range jobIDs {
		if err := o.UpdateJobTags(ctx, id, tags, logger); err == nil {
			count++
		}
	}

	return count, nil
}

// UpdateJobAgent updates the agent provider and model of a job in the pending queue.
func (o *Orchestrator) UpdateJobAgent(ctx context.Context, jobID string, agentProvider string, agentModel string, logger *slog.Logger) error {
	o.mu.Lock()
	job, exists := o.pendingJobs[jobID]
	if !exists {
		o.mu.Unlock()
		// Check if active or completed to return a more specific error
		o.mu.RLock()
		if _, active := o.activeJobs[jobID]; active {
			o.mu.RUnlock()
			return fmt.Errorf("job %s is already active and cannot have agent updated", jobID)
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

	if agentProvider != "" {
		job.WorkItem.AgentProvider = agentProvider
	}
	if agentModel != "" {
		job.WorkItem.AgentModel = agentModel
	}
	o.pendingJobs[jobID] = job

	if o.Persistence != nil {
		o.Persistence.SaveJob(job)
	}

	o.mu.Unlock()
	o.BroadcastEvent("job_agent_updated", job)

	if logger != nil {
		logger.Info("Updated job agent", "jobID", jobID, "agentProvider", agentProvider, "agentModel", agentModel)
	}

	o.evaluatePendingJobs(ctx, logger)
	return nil
}

// UpdateJobMaxRetries updates the maximum retries of a job in the pending queue.
func (o *Orchestrator) UpdateJobMaxRetries(ctx context.Context, jobID string, maxRetries int, logger *slog.Logger) error {
	o.mu.Lock()
	job, exists := o.pendingJobs[jobID]
	if !exists {
		o.mu.Unlock()
		// Check if active or completed to return a more specific error
		o.mu.RLock()
		if _, active := o.activeJobs[jobID]; active {
			o.mu.RUnlock()
			return fmt.Errorf("job %s is already active and cannot have max retries updated", jobID)
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

	// We need to allocate a new int to avoid sharing pointers across jobs
	mr := new(int)
	*mr = maxRetries
	job.WorkItem.MaxRetries = mr
	o.pendingJobs[jobID] = job
	if o.Persistence != nil {
		o.Persistence.SaveJob(job)
	}
	o.mu.Unlock()
	o.BroadcastEvent("job_max_retries_updated", job)

	if logger != nil {
		logger.Info("Updated job max retries", "jobID", jobID, "maxRetries", maxRetries)
	}

	// Do not call evaluatePendingJobs while holding the lock.
	// In fact, updating max retries does not immediately change whether a job should run,
	// but we can call it just to be consistent, but outside the lock.
	o.evaluatePendingJobs(ctx, logger)
	return nil
}

// UpdateJobsMaxRetriesByTag updates the maximum retries of pending jobs that match the given tag.
func (o *Orchestrator) UpdateJobsMaxRetriesByTag(ctx context.Context, tag string, maxRetries int, logger *slog.Logger) (int, error) {
	o.mu.Lock()
	var jobIDs []string
	lowerTag := strings.ToLower(tag)

	for id, job := range o.pendingJobs {
		for _, t := range job.WorkItem.Tags {
			if strings.ToLower(t) == lowerTag {
				jobIDs = append(jobIDs, id)
				break
			}
		}
	}
	o.mu.Unlock()

	count := 0
	for _, id := range jobIDs {
		if err := o.UpdateJobMaxRetries(ctx, id, maxRetries, logger); err == nil {
			count++
		}
	}

	return count, nil
}

// UpdateJobsMaxRetriesByMatch updates the maximum retries of pending jobs that match the given regex.
func (o *Orchestrator) UpdateJobsMaxRetriesByMatch(ctx context.Context, match string, maxRetries int, logger *slog.Logger) (int, error) {
	matcher, err := regexp.Compile("(?i)" + match)
	if err != nil {
		return 0, fmt.Errorf("invalid match regex: %w", err)
	}

	o.mu.Lock()
	var jobIDs []string
	for id, job := range o.pendingJobs {
		if matcher.MatchString(job.Summary) || matcher.MatchString(job.Error) {
			jobIDs = append(jobIDs, id)
		}
	}
	o.mu.Unlock()

	count := 0
	for _, id := range jobIDs {
		if err := o.UpdateJobMaxRetries(ctx, id, maxRetries, logger); err == nil {
			count++
		}
	}

	return count, nil
}

// UpdateJobTimeout updates the timeout of a job in the pending queue.
// UpdateJobsTimeoutByTag updates the timeout of pending jobs that match the given tag.
func (o *Orchestrator) UpdateJobsTimeoutByTag(ctx context.Context, tag string, newTimeout time.Duration, logger *slog.Logger) (int, error) {
	o.mu.Lock()
	var jobIDs []string
	lowerTag := strings.ToLower(tag)

	for id, job := range o.pendingJobs {
		for _, t := range job.WorkItem.Tags {
			if strings.ToLower(t) == lowerTag {
				jobIDs = append(jobIDs, id)
				break
			}
		}
	}
	o.mu.Unlock()

	count := 0
	for _, id := range jobIDs {
		if err := o.UpdateJobTimeout(ctx, id, newTimeout, logger); err == nil {
			count++
		}
	}

	return count, nil
}

// UpdateJobsTimeoutByMatch updates the timeout of pending jobs that match the given regex.
func (o *Orchestrator) UpdateJobsTimeoutByMatch(ctx context.Context, match string, newTimeout time.Duration, logger *slog.Logger) (int, error) {
	matcher, err := regexp.Compile("(?i)" + match)
	if err != nil {
		return 0, fmt.Errorf("invalid match regex: %w", err)
	}

	o.mu.Lock()
	var jobIDs []string
	for id, job := range o.pendingJobs {
		if matcher.MatchString(job.Summary) || matcher.MatchString(job.Error) {
			jobIDs = append(jobIDs, id)
		}
	}
	o.mu.Unlock()

	count := 0
	for _, id := range jobIDs {
		if err := o.UpdateJobTimeout(ctx, id, newTimeout, logger); err == nil {
			count++
		}
	}

	return count, nil
}

func (o *Orchestrator) UpdateJobTimeout(ctx context.Context, jobID string, newTimeout time.Duration, logger *slog.Logger) error {
	o.mu.Lock()
	job, exists := o.pendingJobs[jobID]
	if !exists {
		o.mu.Unlock()
		// Check if active or completed to return a more specific error
		o.mu.RLock()
		if _, active := o.activeJobs[jobID]; active {
			o.mu.RUnlock()
			return fmt.Errorf("job %s is already active and cannot have timeout updated", jobID)
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

	job.WorkItem.Timeout = newTimeout
	o.pendingJobs[jobID] = job
	if o.Persistence != nil {
		o.Persistence.SaveJob(job)
	}
	o.mu.Unlock()
	o.BroadcastEvent("job_timeout_updated", job)

	if logger != nil {
		logger.Info("Updated job timeout", "jobID", jobID, "newTimeout", newTimeout)
	}

	o.evaluatePendingJobs(ctx, logger)
	return nil
}

// UpdateJobWorkItem completely updates the WorkItem of a job in the pending queue.
func (o *Orchestrator) UpdateJobWorkItem(ctx context.Context, jobID string, newItem WorkItem, logger *slog.Logger) error {
	o.mu.Lock()
	job, exists := o.pendingJobs[jobID]
	if !exists {
		o.mu.Unlock()
		// Check if active or completed to return a more specific error
		o.mu.RLock()
		if _, active := o.activeJobs[jobID]; active {
			o.mu.RUnlock()
			return fmt.Errorf("job %s is already active and cannot be updated", jobID)
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

	if newItem.ID != jobID {
		o.mu.Unlock()
		return fmt.Errorf("cannot change job ID from %s to %s", jobID, newItem.ID)
	}

	// Create a backup to revert if a cycle is detected
	oldItem := job.WorkItem

	// Deep copy to prevent mutating the original pointers
	newDeps := make([]string, len(newItem.DependsOn))
	copy(newDeps, newItem.DependsOn)
	newItem.DependsOn = newDeps

	if newItem.EnvVars != nil {
		newEnv := make(map[string]string)
		for k, v := range newItem.EnvVars {
			newEnv[k] = v
		}
		newItem.EnvVars = newEnv
	}

	if newItem.Tags != nil {
		newTags := make([]string, len(newItem.Tags))
		copy(newTags, newItem.Tags)
		newItem.Tags = newTags
	}

	job.WorkItem = newItem
	job.Summary = newItem.Summary

	// Update the map temporarily for cycle detection
	o.pendingJobs[jobID] = job

	if err := o.checkCircularDependencyLocked(newItem); err != nil {
		// Revert
		job.WorkItem = oldItem
		job.Summary = oldItem.Summary
		o.pendingJobs[jobID] = job
		o.mu.Unlock()
		return err
	}

	if o.Persistence != nil {
		o.Persistence.SaveJob(job)
	}

	o.mu.Unlock()
	o.BroadcastEvent("job_workitem_updated", job)

	if logger != nil {
		logger.Info("Updated job work item", "jobID", jobID)
	}

	o.evaluatePendingJobs(ctx, logger)
	return nil
}

// HoldJobsByTag holds pending jobs that match the given tag.
func (o *Orchestrator) HoldJobsByTag(ctx context.Context, tag string, logger *slog.Logger) (int, error) {
	o.mu.Lock()
	count := 0
	lowerTag := strings.ToLower(tag)

	for id, job := range o.pendingJobs {
		hasTag := false
		for _, t := range job.WorkItem.Tags {
			if strings.ToLower(t) == lowerTag {
				hasTag = true
				break
			}
		}

		if hasTag && !job.WorkItem.Hold {
			job.WorkItem.Hold = true
			o.pendingJobs[id] = job
			if o.Persistence != nil {
				o.Persistence.SaveJob(job)
			}
			count++
			o.BroadcastEvent("job_held", job)
			if logger != nil {
				logger.Info("Held job", "jobID", id)
			}
		}
	}
	o.mu.Unlock()

	if logger != nil && count > 0 {
		logger.Info("Held jobs by tag", "tag", tag, "count", count)
	}

	return count, nil
}

// HoldJobsByMatch holds pending jobs where the summary matches the given regex.
func (o *Orchestrator) HoldJobsByMatch(ctx context.Context, match string, logger *slog.Logger) (int, error) {
	matcher, err := regexp.Compile("(?i)" + match)
	if err != nil {
		return 0, fmt.Errorf("invalid match regex: %w", err)
	}

	o.mu.Lock()
	count := 0

	for id, job := range o.pendingJobs {
		if (matcher.MatchString(job.Summary) || matcher.MatchString(job.Error)) && !job.WorkItem.Hold {
			job.WorkItem.Hold = true
			o.pendingJobs[id] = job
			if o.Persistence != nil {
				o.Persistence.SaveJob(job)
			}
			count++
			o.BroadcastEvent("job_held", job)
			if logger != nil {
				logger.Info("Held job", "jobID", id)
			}
		}
	}
	o.mu.Unlock()

	if logger != nil && count > 0 {
		logger.Info("Held jobs by match", "match", match, "count", count)
	}

	return count, nil
}

// RenameJob renames a pending job and cascades the ID change to dependent jobs.
func (o *Orchestrator) RenameJob(ctx context.Context, oldID, newID string, logger *slog.Logger) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if oldID == newID {
		return nil
	}

	// Ensure old job exists in pendingJobs
	job, exists := o.pendingJobs[oldID]
	if !exists {
		return fmt.Errorf("job %s not found in pending queue", oldID)
	}

	// Ensure new ID does not already exist
	if _, ok := o.pendingJobs[newID]; ok {
		return fmt.Errorf("job with new ID %s already exists in pending queue", newID)
	}
	for id := range o.activeJobs {
		if id == newID {
			return fmt.Errorf("job with new ID %s already active", newID)
		}
	}
	for _, cJob := range o.completedJobs {
		if cJob.ID == newID {
			return fmt.Errorf("job with new ID %s already completed", newID)
		}
	}

	// Restart delay timer if it exists before we modify job
	if t, ok := o.delayTimers[oldID]; ok {
		t.Stop()
		delete(o.delayTimers, oldID)

		// Re-schedule the delay timer for the new ID
		remaining := time.Until(job.WorkItem.RunAfter)
		if remaining > 0 {
			o.delayTimers[newID] = time.AfterFunc(remaining, func() {
				o.mu.Lock()
				defer o.mu.Unlock()
				if j, ok := o.pendingJobs[newID]; ok {
					j.WorkItem.RunAfter = time.Time{}
					o.pendingJobs[newID] = j
				}
				delete(o.delayTimers, newID)
				o.ForcePoll()
			})
		} else {
			job.WorkItem.RunAfter = time.Time{}
		}
	}

	// Update the job itself
	job.ID = newID
	job.WorkItem.ID = newID

	// Move in map
	delete(o.pendingJobs, oldID)
	o.pendingJobs[newID] = job

	// Cascade dependency rename
	for pid, pJob := range o.pendingJobs {
		updated := false
		for i, dep := range pJob.WorkItem.DependsOn {
			if dep == oldID {
				pJob.WorkItem.DependsOn[i] = newID
				updated = true
			}
		}
		if updated {
			o.pendingJobs[pid] = pJob
			if o.Persistence != nil {
				o.Persistence.SaveJob(pJob)
			}
		}
	}

	if logger != nil {
		logger.Info("Job renamed", "old_id", oldID, "new_id", newID)
	}
	o.BroadcastEvent("job_renamed", map[string]interface{}{
		"old_id": oldID,
		"new_id": newID,
	})

	if o.Persistence != nil {
		o.Persistence.PurgeJob(oldID)
		o.Persistence.SaveJob(job)
	}

	return nil
}

// HoldJob holds a pending job.
func (o *Orchestrator) HoldJob(ctx context.Context, jobID string, logger *slog.Logger) error {
	o.mu.Lock()
	job, exists := o.pendingJobs[jobID]
	if !exists {
		// Check if active or completed to return a more specific error
		if _, active := o.activeJobs[jobID]; active {
			o.mu.Unlock()
			return fmt.Errorf("job %s is already active and cannot be held", jobID)
		}
		for _, completed := range o.completedJobs {
			if completed.ID == jobID {
				o.mu.Unlock()
				return fmt.Errorf("job %s is already completed", jobID)
			}
		}
		o.mu.Unlock()
		return fmt.Errorf("job %s not found in pending queue", jobID)
	}

	job.WorkItem.Hold = true
	o.pendingJobs[jobID] = job
	if o.Persistence != nil {
		o.Persistence.SaveJob(job)
	}
	o.mu.Unlock()
	o.BroadcastEvent("job_held", job)

	if logger != nil {
		logger.Info("Held job", "jobID", jobID)
	}

	return nil
}

// UnholdJobsByTag unholds pending jobs that match the given tag.
func (o *Orchestrator) UnholdJobsByTag(ctx context.Context, tag string, logger *slog.Logger) (int, error) {
	o.mu.Lock()
	count := 0
	lowerTag := strings.ToLower(tag)

	for id, job := range o.pendingJobs {
		hasTag := false
		for _, t := range job.WorkItem.Tags {
			if strings.ToLower(t) == lowerTag {
				hasTag = true
				break
			}
		}

		if hasTag && job.WorkItem.Hold {
			job.WorkItem.Hold = false
			o.pendingJobs[id] = job
			if o.Persistence != nil {
				o.Persistence.SaveJob(job)
			}
			count++
			o.BroadcastEvent("job_unheld", job)
			if logger != nil {
				logger.Info("Unheld job", "jobID", id)
			}
		}
	}
	o.mu.Unlock()

	if logger != nil && count > 0 {
		logger.Info("Unheld jobs by tag", "tag", tag, "count", count)
	}

	if count > 0 {
		o.evaluatePendingJobs(ctx, logger)
	}

	return count, nil
}

// UnholdJobsByMatch unholds pending jobs where the summary matches the given regex.
func (o *Orchestrator) UnholdJobsByMatch(ctx context.Context, match string, logger *slog.Logger) (int, error) {
	matcher, err := regexp.Compile("(?i)" + match)
	if err != nil {
		return 0, fmt.Errorf("invalid match regex: %w", err)
	}

	o.mu.Lock()
	count := 0

	for id, job := range o.pendingJobs {
		if (matcher.MatchString(job.Summary) || matcher.MatchString(job.Error)) && job.WorkItem.Hold {
			job.WorkItem.Hold = false
			o.pendingJobs[id] = job
			if o.Persistence != nil {
				o.Persistence.SaveJob(job)
			}
			count++
			o.BroadcastEvent("job_unheld", job)
			if logger != nil {
				logger.Info("Unheld job", "jobID", id)
			}
		}
	}
	o.mu.Unlock()

	if logger != nil && count > 0 {
		logger.Info("Unheld jobs by match", "match", match, "count", count)
	}

	if count > 0 {
		o.evaluatePendingJobs(ctx, logger)
	}

	return count, nil
}

// UnholdJob unholds a pending job.
func (o *Orchestrator) UnholdJob(ctx context.Context, jobID string, logger *slog.Logger) error {
	o.mu.Lock()
	job, exists := o.pendingJobs[jobID]
	if !exists {
		// Check if active or completed to return a more specific error
		if _, active := o.activeJobs[jobID]; active {
			o.mu.Unlock()
			return fmt.Errorf("job %s is already active and cannot be unheld", jobID)
		}
		for _, completed := range o.completedJobs {
			if completed.ID == jobID {
				o.mu.Unlock()
				return fmt.Errorf("job %s is already completed", jobID)
			}
		}
		o.mu.Unlock()
		return fmt.Errorf("job %s not found in pending queue", jobID)
	}

	job.WorkItem.Hold = false
	o.pendingJobs[jobID] = job
	if o.Persistence != nil {
		o.Persistence.SaveJob(job)
	}
	o.mu.Unlock()
	o.BroadcastEvent("job_unheld", job)

	if logger != nil {
		logger.Info("Unheld job", "jobID", jobID)
	}

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
	if o.Persistence != nil {
		o.Persistence.SaveJob(job)
	}
	o.mu.Unlock()
	o.BroadcastEvent("job_priority_updated", job)

	if logger != nil {
		logger.Info("Updated job priority", "jobID", jobID, "newPriority", newPriority)
	}

	o.evaluatePendingJobs(ctx, logger)
	return nil
}

// UpdateJobsPriorityByTag updates the priority of pending jobs that have a specific tag.
func (o *Orchestrator) UpdateJobsPriorityByTag(ctx context.Context, tag string, newPriority int, logger *slog.Logger) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	lowerTag := strings.ToLower(tag)
	updatedCount := 0

	for id, job := range o.pendingJobs {
		hasTag := false
		for _, t := range job.WorkItem.Tags {
			if strings.ToLower(t) == lowerTag {
				hasTag = true
				break
			}
		}

		if hasTag {
			job.WorkItem.Priority = newPriority
			o.pendingJobs[id] = job
			if o.Persistence != nil {
				o.Persistence.SaveJob(job)
			}
			updatedCount++
			o.BroadcastEvent("job_priority_updated", job)
			if logger != nil {
				logger.Info("Updated job priority by tag", "jobID", id, "tag", tag, "newPriority", newPriority)
			}
		}
	}

	if updatedCount > 0 {
		o.mu.Unlock()
		o.evaluatePendingJobs(ctx, logger)
		o.mu.Lock()
	}
	return updatedCount, nil
}

// UpdateJobsPriorityByMatch updates the priority of pending jobs that match a regular expression.
func (o *Orchestrator) UpdateJobsPriorityByMatch(ctx context.Context, match string, newPriority int, logger *slog.Logger) (int, error) {
	matcher, err := regexp.Compile("(?i)" + match)
	if err != nil {
		return 0, fmt.Errorf("invalid match regex: %w", err)
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	updatedCount := 0

	for id, job := range o.pendingJobs {
		if matcher.MatchString(job.Summary) || matcher.MatchString(job.Error) {
			job.WorkItem.Priority = newPriority
			o.pendingJobs[id] = job
			if o.Persistence != nil {
				o.Persistence.SaveJob(job)
			}
			updatedCount++
			o.BroadcastEvent("job_priority_updated", job)
			if logger != nil {
				logger.Info("Updated job priority by match", "jobID", id, "match", match, "newPriority", newPriority)
			}
		}
	}

	if updatedCount > 0 {
		o.mu.Unlock()
		o.evaluatePendingJobs(ctx, logger)
		o.mu.Lock()
	}
	return updatedCount, nil
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

// RetryJobDownstream resubmits a completed job and all its transitive downstream dependencies.
func (o *Orchestrator) RetryJobDownstream(ctx context.Context, jobID string, logger *slog.Logger) ([]string, error) {
	o.mu.RLock()

	// 1. Verify the root job is not currently active or pending
	if _, active := o.activeJobs[jobID]; active {
		o.mu.RUnlock()
		return nil, fmt.Errorf("job %s is active", jobID)
	}
	if _, pending := o.pendingJobs[jobID]; pending {
		o.mu.RUnlock()
		return nil, fmt.Errorf("job %s is pending", jobID)
	}

	// 2. Fetch all completed jobs from memory and persistence to build the graph
	var allCompleted []JobInfo
	if o.Persistence != nil {
		if pJobs, err := o.Persistence.GetJobs(10000); err == nil {
			allCompleted = pJobs
		} else {
			allCompleted = o.completedJobs
		}
	} else {
		allCompleted = make([]JobInfo, len(o.completedJobs))
		copy(allCompleted, o.completedJobs)
	}

	// 3. Build adjacency list for downstream dependencies (job -> jobs that depend on it)
	// and map of ID to WorkItem for fast lookup
	downstreamMap := make(map[string][]string)
	workItemMap := make(map[string]WorkItem)
	foundRoot := false

	for _, job := range allCompleted {
		workItemMap[job.ID] = job.WorkItem
		if job.ID == jobID {
			foundRoot = true
		}
		for _, dep := range job.WorkItem.DependsOn {
			downstreamMap[dep] = append(downstreamMap[dep], job.ID)
		}
	}

	if !foundRoot {
		o.mu.RUnlock()
		return nil, fmt.Errorf("job %s not found in history", jobID)
	}

	// 4. Find all transitive downstream jobs using BFS
	visited := make(map[string]bool)
	queue := []string{jobID}
	var jobsToRetry []WorkItem

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if visited[curr] {
			continue
		}
		visited[curr] = true

		// Add to retry list if not active/pending
		_, active := o.activeJobs[curr]
		_, pending := o.pendingJobs[curr]
		if !active && !pending {
			if item, exists := workItemMap[curr]; exists {
				jobsToRetry = append(jobsToRetry, item)
			}
		}

		// Enqueue downstream dependents
		if dependents, hasDependents := downstreamMap[curr]; hasDependents {
			queue = append(queue, dependents...)
		}
	}

	o.mu.RUnlock()

	// 5. Resubmit all found jobs
	var retriedIDs []string
	for _, item := range jobsToRetry {
		if logger != nil {
			logger.Info("Retrying job downstream", "id", item.ID, "root", jobID)
		}
		if err := o.processWorkItem(ctx, item, 0, logger); err == nil {
			retriedIDs = append(retriedIDs, item.ID)
		} else {
			if logger != nil {
				logger.Error("Failed to retry job downstream", "id", item.ID, "error", err)
			}
		}
	}

	return retriedIDs, nil
}

// HealJobs attempts to recover failed jobs by appending failure context to their description and resubmitting them.
func (o *Orchestrator) HealJobs(ctx context.Context, match, tag string, logger *slog.Logger) (int, error) {
	var matcher *regexp.Regexp
	var err error
	if match != "" {
		matcher, err = regexp.Compile("(?i)" + match)
		if err != nil {
			return 0, fmt.Errorf("invalid match regex: %w", err)
		}
	}

	lowerTag := strings.ToLower(tag)

	o.mu.RLock()
	var toHeal []JobInfo
	for _, job := range o.completedJobs {
		if job.Status == "Failed" || job.Status == "error" {
			if matcher != nil && !matcher.MatchString(job.Summary) && !matcher.MatchString(job.Error) {
				continue
			}

			if tag != "" {
				hasTag := false
				for _, t := range job.WorkItem.Tags {
					if strings.ToLower(t) == lowerTag {
						hasTag = true
						break
					}
				}
				if !hasTag {
					continue
				}
			}

			// Check if already active
			if _, active := o.activeJobs[job.ID]; !active {
				toHeal = append(toHeal, job)
			}
		}
	}
	o.mu.RUnlock()

	count := 0
	for _, job := range toHeal {
		if logger != nil {
			logger.Info("Healing failed job", "id", job.ID)
		}

		// Fetch logs
		var logsText string
		logStream, err := o.GetLogs(ctx, job.ID)
		if err == nil && logStream != nil {
			logBytes, _ := io.ReadAll(logStream)
			logsText = string(logBytes)
			logStream.Close()
		} else if logger != nil {
			logger.Warn("Failed to fetch logs for healing", "id", job.ID, "error", err)
		}

		logLines := strings.Split(logsText, "\n")
		if len(logLines) > 500 {
			logLines = logLines[len(logLines)-500:]
			logsText = "... [Logs Truncated] ...\n" + strings.Join(logLines, "\n")
		}

		newItem := job.WorkItem
		newItem.ID = fmt.Sprintf("%s-healed", job.ID)

		failureContext := fmt.Sprintf("\n\n---\nPrevious Job Failure Context:\nError: %s\nLogs:\n```\n%s\n```\n", job.Error, logsText)
		newItem.Description = newItem.Description + failureContext

		hasAutoHealTag := false
		for _, t := range newItem.Tags {
			if t == "auto-heal" {
				hasAutoHealTag = true
				break
			}
		}
		if !hasAutoHealTag {
			newItem.Tags = append(newItem.Tags, "auto-heal")
		}

		if err := o.SubmitJob(ctx, newItem, logger); err == nil {
			count++
		} else if logger != nil {
			logger.Error("Failed to submit healed job", "id", job.ID, "error", err)
		}
	}
	return count, nil
}

// RetryFailedJobs resubmits all failed jobs from history.
func (o *Orchestrator) RetryFailedJobs(ctx context.Context, match string, tag string, logger *slog.Logger) (int, error) {
	var matcher *regexp.Regexp
	var err error
	if match != "" {
		matcher, err = regexp.Compile(match)
		if err != nil {
			return 0, fmt.Errorf("invalid retry match pattern: %w", err)
		}
	}

	lowerTag := strings.ToLower(tag)

	o.mu.RLock()
	var toRetry []WorkItem
	for _, job := range o.completedJobs {
		if job.Status == "Failed" {
			if matcher != nil && !matcher.MatchString(job.Error) {
				continue
			}

			if tag != "" {
				hasTag := false
				for _, t := range job.WorkItem.Tags {
					if strings.ToLower(t) == lowerTag {
						hasTag = true
						break
					}
				}
				if !hasTag {
					continue
				}
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

// sanitizeEnvVarName is a helper to sanitize job ID for env var name
func sanitizeEnvVarName(id string) string {
	var sb strings.Builder
	sb.Grow(len(id))
	for i := 0; i < len(id); i++ {
		c := id[i]
		if c >= 'a' && c <= 'z' {
			sb.WriteByte(c - 32)
		} else if (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			sb.WriteByte(c)
		} else if c == '-' {
			sb.WriteByte('_')
		} else {
			sb.WriteByte('_')
		}
	}
	return sb.String()
}

// checkDependenciesMetLocked returns:
// - shouldRun bool: whether the job should be executed
// - shouldFail bool: whether the job should immediately fail (e.g. required dependency failed)
// - shouldSkip bool: whether the job should be skipped (e.g. on_failure condition not met)
// - failedDep string: the first dependency that caused a failure, if applicable
// - outputs map[string]string: accumulated outputs from successful dependencies
func (o *Orchestrator) checkDependenciesMetLocked(item WorkItem) (shouldRun bool, shouldFail bool, shouldSkip bool, failedDep string, outputs map[string]string) {
	outputs = make(map[string]string)
	if len(item.DependsOn) == 0 {
		return true, false, false, "", outputs
	}

	failedCount := 0
	completedCount := 0

	// Collect status for all dependencies
	for _, dep := range item.DependsOn {
		var depJob JobInfo
		found := false

		if _, ok := o.activeJobs[dep]; ok {
			return false, false, false, "", nil // Active, not met yet
		}
		if _, ok := o.pendingJobs[dep]; ok {
			return false, false, false, "", nil // Pending, not met yet
		}

		// Check memory history (newest first)
		for i := len(o.completedJobs) - 1; i >= 0; i-- {
			completed := o.completedJobs[i]
			if completed.ID == dep {
				depJob = completed
				found = true
				break
			}
		}

		// Check persistence if not found in memory history
		if !found && o.Persistence != nil {
			job, err := o.Persistence.GetJob(dep)
			if err == nil {
				depJob = *job
				found = true
			}
		}

		if !found {
			// Dependency not found anywhere, wait for it
			return false, false, false, "", nil
		}

		if depJob.Status == "Failed" || depJob.Status == "Canceled" {
			failedCount++
			if failedDep == "" {
				failedDep = dep
			}
		} else if depJob.Status == "Completed" || depJob.Status == "Skipped" {
			completedCount++
		}

		// Accumulate outputs
		prefix := fmt.Sprintf("DEP_%s_", sanitizeEnvVarName(dep))
		for k, v := range depJob.Outputs {
			outputs[prefix+strings.ToUpper(k)] = v
		}
	}

	cond := strings.ToLower(strings.TrimSpace(item.RunCondition))

	switch cond {
	case "always":
		return true, false, false, "", outputs
	case "on_failure":
		if failedCount > 0 {
			return true, false, false, "", outputs
		} else {
			return false, false, true, "", outputs // Should skip
		}
	default: // "on_success" or empty
		if failedCount > 0 {
			return false, true, false, failedDep, outputs // Should fail
		} else {
			return true, false, false, "", outputs
		}
	}
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

	// Concurrency Group logic
	var jobsToCancel []string
	var groupActive bool
	if item.ConcurrencyGroup != "" && retryCount == 0 && !bypassApproval {
		for id, activeJob := range o.activeJobs {
			if activeJob.WorkItem.ConcurrencyGroup == item.ConcurrencyGroup && id != item.ID {
				if item.CancelInProgress {
					jobsToCancel = append(jobsToCancel, id)
				} else {
					groupActive = true
				}
			}
		}
		for id, pendingJob := range o.pendingJobs {
			if pendingJob.WorkItem.ConcurrencyGroup == item.ConcurrencyGroup && id != item.ID {
				if item.CancelInProgress {
					jobsToCancel = append(jobsToCancel, id)
				}
			}
		}
	}

	if groupActive {
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
		o.BroadcastEvent("job_pending", job)
		if logger != nil {
			logger.Info("Job pending due to active concurrency group", "id", item.ID, "group", item.ConcurrencyGroup)
		}
		return nil
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

		if len(jobsToCancel) > 0 {
			if logger != nil {
				logger.Info("Canceling existing jobs in concurrency group for delayed job", "group", item.ConcurrencyGroup, "jobs", jobsToCancel)
			}
			for _, cancelID := range jobsToCancel {
				if err := o.CancelJob(ctx, cancelID); err != nil {
					if logger != nil {
						logger.Warn("Failed to cancel job in concurrency group", "jobID", cancelID, "error", err)
					}
				}
			}
		}

		time.AfterFunc(delay, func() {
			o.evaluatePendingJobs(context.Background(), logger)
		})

		return nil
	}

	requireApproval := o.RequireApproval
	if item.RequireApproval != nil {
		requireApproval = *item.RequireApproval
	}

	if requireApproval && !bypassApproval {
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

		if len(jobsToCancel) > 0 {
			if logger != nil {
				logger.Info("Canceling existing jobs in concurrency group for pending approval job", "group", item.ConcurrencyGroup, "jobs", jobsToCancel)
			}
			for _, cancelID := range jobsToCancel {
				if err := o.CancelJob(ctx, cancelID); err != nil {
					if logger != nil {
						logger.Warn("Failed to cancel job in concurrency group", "jobID", cancelID, "error", err)
					}
				}
			}
		}

		o.BroadcastEvent("job_pending_approval", job)
		if logger != nil {
			logger.Info("Job pending approval", "id", item.ID)
		}
		return nil
	}

	var depOutputs map[string]string
	if len(item.DependsOn) > 0 {
		var shouldRun, shouldFail, shouldSkip bool
		var failedDep string
		shouldRun, shouldFail, shouldSkip, failedDep, depOutputs = o.checkDependenciesMetLocked(item)

		if shouldFail {
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

			if len(jobsToCancel) > 0 {
				if logger != nil {
					logger.Info("Canceling existing jobs in concurrency group despite dependency failure", "group", item.ConcurrencyGroup, "jobs", jobsToCancel)
				}
				for _, cancelID := range jobsToCancel {
					if err := o.CancelJob(ctx, cancelID); err != nil {
						if logger != nil {
							logger.Warn("Failed to cancel job in concurrency group", "jobID", cancelID, "error", err)
						}
					}
				}
			}

			o.BroadcastEvent("job_failed", job)
			if logger != nil {
				logger.Error("Job failed due to failed dependency", "id", item.ID, "dependency", failedDep)
			}
			return fmt.Errorf("dependency %s failed", failedDep)
		} else if shouldSkip {
			// Condition not met (e.g. on_failure but all succeeded), skip job
			job := JobInfo{
				ID:        item.ID,
				Summary:   item.Summary,
				StartTime: time.Now(),
				EndTime:   time.Now(),
				Status:    "Skipped",
				Error:     "Skipped due to run condition not met",
				WorkItem:  item,
			}
			o.addToHistory(job, logger)
			o.mu.Unlock()

			if len(jobsToCancel) > 0 {
				if logger != nil {
					logger.Info("Canceling existing jobs in concurrency group despite job skip", "group", item.ConcurrencyGroup, "jobs", jobsToCancel)
				}
				for _, cancelID := range jobsToCancel {
					if err := o.CancelJob(ctx, cancelID); err != nil {
						if logger != nil {
							logger.Warn("Failed to cancel job in concurrency group", "jobID", cancelID, "error", err)
						}
					}
				}
			}

			o.BroadcastEvent("job_skipped", job)
			if logger != nil {
				logger.Info("Job skipped due to run condition", "id", item.ID, "run_condition", item.RunCondition)
			}

			// We need to unblock dependents of this skipped job
			o.evaluatePendingJobs(ctx, logger)

			return nil
		} else if !shouldRun {
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

			if len(jobsToCancel) > 0 {
				if logger != nil {
					logger.Info("Canceling existing jobs in concurrency group for pending dep job", "group", item.ConcurrencyGroup, "jobs", jobsToCancel)
				}
				for _, cancelID := range jobsToCancel {
					if err := o.CancelJob(ctx, cancelID); err != nil {
						if logger != nil {
							logger.Warn("Failed to cancel job in concurrency group", "jobID", cancelID, "error", err)
						}
					}
				}
			}

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

	// Merge dependency outputs into the new job's environment variables
	if len(depOutputs) > 0 {
		if item.EnvVars == nil {
			item.EnvVars = make(map[string]string)
		}
		// Deep copy so we don't mutate shared structures incorrectly if retried
		newEnv := make(map[string]string)
		for k, v := range item.EnvVars {
			newEnv[k] = v
		}
		for k, v := range depOutputs {
			newEnv[k] = v
		}
		item.EnvVars = newEnv
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

	if len(jobsToCancel) > 0 {
		if logger != nil {
			logger.Info("Canceling existing jobs in concurrency group", "group", item.ConcurrencyGroup, "jobs", jobsToCancel)
		}
		for _, cancelID := range jobsToCancel {
			if err := o.CancelJob(ctx, cancelID); err != nil {
				if logger != nil {
					logger.Warn("Failed to cancel job in concurrency group", "jobID", cancelID, "error", err)
				}
			}
		}
	}

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
				maxRetries := o.MaxRetries
				if job.WorkItem.MaxRetries != nil {
					maxRetries = *job.WorkItem.MaxRetries
				}
				if maxRetries > 0 && job.RetryCount < maxRetries && spawnCtx.Err() != context.DeadlineExceeded && spawnCtx.Err() != context.Canceled {
					job.RetryCount++
					job.Status = "Retrying"
					job.Error = spawnErr.Error()

					delay := o.RetryDelay
					if job.WorkItem.RetryDelay != nil {
						delay = *job.WorkItem.RetryDelay
					}

					multiplier := 1.0
					if job.WorkItem.RetryBackoffMultiplier != nil {
						multiplier = *job.WorkItem.RetryBackoffMultiplier
					}

					// Apply exponential backoff based on multiplier
					for i := 1; i < job.RetryCount; i++ {
						delay = time.Duration(float64(delay) * multiplier)
					}

					job.RetryAfter = time.Now().Add(delay)
					o.pendingJobs[item.ID] = job
					finalJob = job
					hasFinalJob = true

					if logger != nil {
						logger.Info("Job failed, scheduling auto-retry", "id", item.ID, "attempt", job.RetryCount, "max", maxRetries, "delay", delay)
					}

					// Trigger re-evaluation when delay expires
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

				if spawnJobsJSON, ok := finalJob.Outputs["RECAC_SPAWN_JOBS"]; ok {
					var newJobs []WorkItem
					if err := json.Unmarshal([]byte(spawnJobsJSON), &newJobs); err == nil {
						for _, nj := range newJobs {
							if err := o.SubmitJob(context.Background(), nj, logger); err != nil {
								if logger != nil {
									logger.Error("Failed to dynamically spawn job", "parent", item.ID, "new_job", nj.ID, "error", err)
								}
							} else {
								if logger != nil {
									logger.Info("Dynamically spawned job", "parent", item.ID, "new_job", nj.ID)
								}
							}
						}
					} else if logger != nil {
						logger.Error("Failed to parse RECAC_SPAWN_JOBS output", "parent", item.ID, "error", err)
					}
				}
			}
		}

		if o.notifier != nil {
			if spawnErr != nil {
				_, _ = o.notifier.Notify(ctx, "on_failure", fmt.Sprintf("Job %s failed: %v", item.ID, spawnErr), threadState)
			} else {
				_, _ = o.notifier.Notify(ctx, "on_success", fmt.Sprintf("Job %s completed successfully", item.ID), threadState)
			}
		}

		if hasFinalJob && finalJob.WorkItem.WebhookURL != "" && (finalJob.Status == "Completed" || finalJob.Status == "Failed" || finalJob.Status == "Canceled") {
			go o.fireJobWebhook(finalJob, logger)
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
		o.recordSpawnFailure(logger)
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
		o.recordSpawnSuccess()
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

		requireApproval := o.RequireApproval
		if jobInfo.WorkItem.RequireApproval != nil {
			requireApproval = *jobInfo.WorkItem.RequireApproval
		}

		// If require approval is true, and it hasn't been approved, skip it.
		// If it has been approved, we process it normally (check dependencies, spawn)
		if requireApproval && !jobInfo.Approved {
			continue
		}

		if jobInfo.WorkItem.Hold {
			continue
		}

		item := jobInfo.WorkItem
		shouldRun, shouldFail, shouldSkip, failedDep, _ := o.checkDependenciesMetLocked(item)

		if shouldRun {
			if item.Delay > 0 {
				// Apply delay by setting RunAfter and resetting Delay so it only triggers once
				item.RunAfter = time.Now().Add(item.Delay)
				item.Delay = 0
				jobInfo.WorkItem = item
				o.pendingJobs[id] = jobInfo

				if logger != nil {
					logger.Info("Job dependencies met, applying delay", "id", item.ID, "delay", item.RunAfter.Sub(time.Now()))
				}

				// Re-evaluate when the delay expires
				delay := time.Until(item.RunAfter)
				if delay < 0 {
					delay = 0
				}
				timer := time.AfterFunc(delay, func() {
					o.evaluatePendingJobs(context.Background(), logger)
				})
				o.delayTimers[id] = timer
			} else {
				if t, ok := o.delayTimers[id]; ok {
					t.Stop()
					delete(o.delayTimers, id)
				}
				toProcess = append(toProcess, pendingJob{item: item, retryCount: jobInfo.RetryCount, approved: jobInfo.Approved})
				delete(o.pendingJobs, id)
			}
		} else if shouldFail {
			if t, ok := o.delayTimers[id]; ok {
				t.Stop()
				delete(o.delayTimers, id)
			}
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
		} else if shouldSkip {
			if t, ok := o.delayTimers[id]; ok {
				t.Stop()
				delete(o.delayTimers, id)
			}
			// Condition not met (e.g. on_failure but all succeeded), skip job
			delete(o.pendingJobs, id)
			job := JobInfo{
				ID:        item.ID,
				Summary:   item.Summary,
				StartTime: time.Now(),
				EndTime:   time.Now(),
				Status:    "Skipped",
				Error:     "Skipped due to run condition not met",
				WorkItem:  item,
			}
			o.addToHistory(job, logger)
			// Ensure we broadcast after dropping the lock to avoid deadlocks
			defer func() {
				o.BroadcastEvent("job_skipped", job)
				// trigger re-evaluation for anything depending on this skipped job
				o.evaluatePendingJobs(ctx, logger)
			}()
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

	// Build a map of currently active concurrency groups to prevent multiple jobs in the same group
	// from starting in the same tick if CancelInProgress is false.
	activeGroups := make(map[string]bool)
	o.mu.Lock()
	for _, activeJob := range o.activeJobs {
		if activeJob.WorkItem.ConcurrencyGroup != "" {
			activeGroups[activeJob.WorkItem.ConcurrencyGroup] = true
		}
	}
	o.mu.Unlock()

	var finalProcess []pendingJob
	for _, pJob := range toProcess {
		item := pJob.item
		if item.ConcurrencyGroup != "" && !item.CancelInProgress {
			if activeGroups[item.ConcurrencyGroup] {
				// Re-add to pending jobs, it must wait for the current job in the group to finish
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
				continue
			}
			// Mark this group as active so subsequent pending jobs in the same group wait
			activeGroups[item.ConcurrencyGroup] = true
		}
		finalProcess = append(finalProcess, pJob)
	}

	for _, pJob := range finalProcess {
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

// UpdateJobProgress updates the progress and status message of a job.
func (o *Orchestrator) UpdateJobProgress(jobID string, progress *int, statusMessage *string, logger *slog.Logger) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	// 1. Check active jobs
	if job, ok := o.activeJobs[jobID]; ok {
		if progress != nil {
			job.Progress = progress
		}
		if statusMessage != nil {
			job.StatusMessage = statusMessage
		}
		o.activeJobs[jobID] = job
		if logger != nil {
			logger.Info("Updated progress for active job", "jobID", jobID, "progress", progress, "statusMessage", statusMessage)
		}
		o.BroadcastEvent("job_progress_updated", job)
		return nil
	}

	// 2. Check pending jobs
	if job, ok := o.pendingJobs[jobID]; ok {
		if progress != nil {
			job.Progress = progress
		}
		if statusMessage != nil {
			job.StatusMessage = statusMessage
		}
		o.pendingJobs[jobID] = job
		if logger != nil {
			logger.Info("Updated progress for pending job", "jobID", jobID, "progress", progress, "statusMessage", statusMessage)
		}
		o.BroadcastEvent("job_progress_updated", job)
		return nil
	}

	// 3. Check memory history
	foundInMemory := false
	for i := len(o.completedJobs) - 1; i >= 0; i-- {
		if o.completedJobs[i].ID == jobID {
			if progress != nil {
				o.completedJobs[i].Progress = progress
			}
			if statusMessage != nil {
				o.completedJobs[i].StatusMessage = statusMessage
			}

			// 4. Also update persistence if found in memory
			if o.Persistence != nil {
				if err := o.Persistence.SaveJob(o.completedJobs[i]); err != nil && logger != nil {
					logger.Warn("Failed to persist updated job progress", "jobID", jobID, "error", err)
				}
			}

			foundInMemory = true
			if logger != nil {
				logger.Info("Updated progress for completed job (in memory)", "jobID", jobID, "progress", progress, "statusMessage", statusMessage)
			}
			o.BroadcastEvent("job_progress_updated", o.completedJobs[i])
			break
		}
	}

	if foundInMemory {
		return nil
	}

	// 5. Check persistence only if not in memory
	if o.Persistence != nil {
		job, err := o.Persistence.GetJob(jobID)
		if err == nil {
			if progress != nil {
				job.Progress = progress
			}
			if statusMessage != nil {
				job.StatusMessage = statusMessage
			}
			if err := o.Persistence.SaveJob(*job); err != nil {
				return fmt.Errorf("failed to persist updated job progress: %w", err)
			}
			if logger != nil {
				logger.Info("Updated progress for completed job (in persistence)", "jobID", jobID, "progress", progress, "statusMessage", statusMessage)
			}
			o.BroadcastEvent("job_progress_updated", *job)
			return nil
		}
	}

	return fmt.Errorf("job %s not found", jobID)
}

// AddJobMetrics sets metrics variables for a specific job.
func (o *Orchestrator) AddJobMetrics(jobID string, metrics map[string]float64, logger *slog.Logger) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	// 1. Check active jobs
	if job, ok := o.activeJobs[jobID]; ok {
		if job.Metrics == nil {
			job.Metrics = make(map[string]float64)
		}
		for k, v := range metrics {
			job.Metrics[k] += v
		}
		o.activeJobs[jobID] = job
		if logger != nil {
			logger.Info("Added metrics for active job", "jobID", jobID, "metrics", len(metrics))
		}
		return nil
	}

	// 2. Check pending jobs (though unlikely to have metrics set)
	if job, ok := o.pendingJobs[jobID]; ok {
		if job.Metrics == nil {
			job.Metrics = make(map[string]float64)
		}
		for k, v := range metrics {
			job.Metrics[k] += v
		}
		o.pendingJobs[jobID] = job
		if logger != nil {
			logger.Info("Added metrics for pending job", "jobID", jobID, "metrics", len(metrics))
		}
		return nil
	}

	// 3. Check memory history
	foundInMemory := false
	for i := len(o.completedJobs) - 1; i >= 0; i-- {
		if o.completedJobs[i].ID == jobID {
			if o.completedJobs[i].Metrics == nil {
				o.completedJobs[i].Metrics = make(map[string]float64)
			}
			for k, v := range metrics {
				o.completedJobs[i].Metrics[k] += v
			}

			// 4. Also update persistence if found in memory
			if o.Persistence != nil {
				if err := o.Persistence.SaveJob(o.completedJobs[i]); err != nil && logger != nil {
					logger.Warn("Failed to persist updated job metrics", "jobID", jobID, "error", err)
				}
			}

			foundInMemory = true
			if logger != nil {
				logger.Info("Added metrics for completed job (in memory)", "jobID", jobID, "metrics", len(metrics))
			}
			break
		}
	}

	if foundInMemory {
		return nil
	}

	// 5. Check persistence only if not in memory
	if o.Persistence != nil {
		job, err := o.Persistence.GetJob(jobID)
		if err == nil {
			if job.Metrics == nil {
				job.Metrics = make(map[string]float64)
			}
			for k, v := range metrics {
				job.Metrics[k] += v
			}
			if err := o.Persistence.SaveJob(*job); err != nil {
				return fmt.Errorf("failed to persist updated job metrics: %w", err)
			}
			if logger != nil {
				logger.Info("Added metrics for completed job (in persistence)", "jobID", jobID, "metrics", len(metrics))
			}
			return nil
		}
	}

	return fmt.Errorf("job %s not found", jobID)
}

// SetJobOutput sets output variables for a specific job.
func (o *Orchestrator) SetJobOutput(jobID string, outputs map[string]string, logger *slog.Logger) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	// 1. Check active jobs
	if job, ok := o.activeJobs[jobID]; ok {
		if job.Outputs == nil {
			job.Outputs = make(map[string]string)
		}
		for k, v := range outputs {
			job.Outputs[k] = v
		}
		o.activeJobs[jobID] = job
		if logger != nil {
			logger.Info("Set output for active job", "jobID", jobID, "outputs", len(outputs))
		}
		return nil
	}

	// 2. Check pending jobs (though unlikely to have outputs set)
	if job, ok := o.pendingJobs[jobID]; ok {
		if job.Outputs == nil {
			job.Outputs = make(map[string]string)
		}
		for k, v := range outputs {
			job.Outputs[k] = v
		}
		o.pendingJobs[jobID] = job
		if logger != nil {
			logger.Info("Set output for pending job", "jobID", jobID, "outputs", len(outputs))
		}
		return nil
	}

	// 3. Check memory history
	foundInMemory := false
	for i := len(o.completedJobs) - 1; i >= 0; i-- {
		if o.completedJobs[i].ID == jobID {
			if o.completedJobs[i].Outputs == nil {
				o.completedJobs[i].Outputs = make(map[string]string)
			}
			for k, v := range outputs {
				o.completedJobs[i].Outputs[k] = v
			}

			// 4. Also update persistence if found in memory
			if o.Persistence != nil {
				if err := o.Persistence.SaveJob(o.completedJobs[i]); err != nil && logger != nil {
					logger.Warn("Failed to persist updated job output", "jobID", jobID, "error", err)
				}
			}

			foundInMemory = true
			if logger != nil {
				logger.Info("Set output for completed job (in memory)", "jobID", jobID, "outputs", len(outputs))
			}
			break
		}
	}

	if foundInMemory {
		return nil
	}

	// 5. Check persistence only if not in memory
	if o.Persistence != nil {
		job, err := o.Persistence.GetJob(jobID)
		if err == nil {
			if job.Outputs == nil {
				job.Outputs = make(map[string]string)
			}
			for k, v := range outputs {
				job.Outputs[k] = v
			}
			if err := o.Persistence.SaveJob(*job); err != nil {
				return fmt.Errorf("failed to persist updated job output: %w", err)
			}
			if logger != nil {
				logger.Info("Set output for completed job (in persistence)", "jobID", jobID, "outputs", len(outputs))
			}
			return nil
		}
	}

	return fmt.Errorf("job %s not found", jobID)
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

// PurgeJobsByStatus purges all jobs matching a specific status from both memory and persistence.
func (o *Orchestrator) PurgeJobsByStatus(status string, logger *slog.Logger) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	lowerStatus := strings.ToLower(status)

	purgedIDs := make(map[string]bool)

	// 1. Purge from memory
	var newCompleted []JobInfo
	for _, job := range o.completedJobs {
		if strings.ToLower(job.Status) == lowerStatus {
			purgedIDs[job.ID] = true
			if logger != nil {
				logger.Info("Job purged from history by status", "id", job.ID, "status", status)
			}
			o.BroadcastEvent("job_purged", map[string]string{"id": job.ID})
		} else {
			newCompleted = append(newCompleted, job)
		}
	}
	o.completedJobs = newCompleted

	// 2. Purge from persistence
	if o.Persistence != nil {
		jobsInDb, err := o.Persistence.GetJobs(10000)
		if err == nil {
			for _, job := range jobsInDb {
				if strings.ToLower(job.Status) == lowerStatus {
					if err := o.Persistence.PurgeJob(job.ID); err == nil {
						if !purgedIDs[job.ID] {
							purgedIDs[job.ID] = true
							if logger != nil {
								logger.Info("Job purged from history by status (DB only)", "id", job.ID, "status", status)
							}
							o.BroadcastEvent("job_purged", map[string]string{"id": job.ID})
						}
					}
				}
			}
		}
	}

	return len(purgedIDs), nil
}

// PurgeJobsByMatch purges all completed/failed jobs matching a regex from both memory and persistence.
func (o *Orchestrator) PurgeJobsByMatch(match string, logger *slog.Logger) (int, error) {
	matcher, err := regexp.Compile("(?i)" + match)
	if err != nil {
		return 0, fmt.Errorf("invalid match regex: %w", err)
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	purgedIDs := make(map[string]bool)

	// 1. Purge from memory
	var newCompleted []JobInfo
	for _, job := range o.completedJobs {
		if matcher.MatchString(job.Summary) || matcher.MatchString(job.Error) {
			purgedIDs[job.ID] = true
			if logger != nil {
				logger.Info("Job purged from history by match", "id", job.ID, "match", match)
			}
			o.BroadcastEvent("job_purged", map[string]string{"id": job.ID})
		} else {
			newCompleted = append(newCompleted, job)
		}
	}
	o.completedJobs = newCompleted

	// 2. Purge from persistence
	if o.Persistence != nil {
		jobsInDb, err := o.Persistence.GetJobs(10000)
		if err == nil {
			for _, job := range jobsInDb {
				if matcher.MatchString(job.Summary) || matcher.MatchString(job.Error) {
					if err := o.Persistence.PurgeJob(job.ID); err == nil {
						if !purgedIDs[job.ID] {
							purgedIDs[job.ID] = true
							if logger != nil {
								logger.Info("Job purged from history by match (DB only)", "id", job.ID, "match", match)
							}
							o.BroadcastEvent("job_purged", map[string]string{"id": job.ID})
						}
					}
				}
			}
		}
	}

	return len(purgedIDs), nil
}

// PurgeJobsByTag purges all completed/failed jobs that have the specified tag from both memory and persistence.
func (o *Orchestrator) PurgeJobsByTag(tag string, logger *slog.Logger) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	lowerTag := strings.ToLower(tag)

	purgedIDs := make(map[string]bool)

	// 1. Purge from memory
	var newCompleted []JobInfo
	for _, job := range o.completedJobs {
		hasTag := false
		for _, t := range job.WorkItem.Tags {
			if strings.ToLower(t) == lowerTag {
				hasTag = true
				break
			}
		}

		if hasTag {
			purgedIDs[job.ID] = true
			if logger != nil {
				logger.Info("Job purged from history by tag", "id", job.ID, "tag", tag)
			}
			o.BroadcastEvent("job_purged", map[string]string{"id": job.ID})
		} else {
			newCompleted = append(newCompleted, job)
		}
	}
	o.completedJobs = newCompleted

	// 2. Purge from persistence
	if o.Persistence != nil {
		// As there is no PurgeJobsByTag method in the Persistence interface,
		// we fetch all jobs, check the tags, and delete them individually.
		// For a better approach in the future, a new method could be added to Persistence.
		jobsInDb, err := o.Persistence.GetJobs(10000) // Getting a large number to ensure we get all
		if err == nil {
			for _, job := range jobsInDb {
				hasTag := false
				for _, t := range job.WorkItem.Tags {
					if strings.ToLower(t) == lowerTag {
						hasTag = true
						break
					}
				}

				if hasTag {
					if err := o.Persistence.PurgeJob(job.ID); err == nil {
						if !purgedIDs[job.ID] {
							purgedIDs[job.ID] = true
							if logger != nil {
								logger.Info("Job purged from history by tag (DB only)", "id", job.ID, "tag", tag)
							}
							o.BroadcastEvent("job_purged", map[string]string{"id": job.ID})
						}
					}
				}
			}
		}
	}

	return len(purgedIDs), nil
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
			o.mu.Lock()
			for id, t := range o.delayTimers {
				t.Stop()
				delete(o.delayTimers, id)
			}
			o.mu.Unlock()
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

// UpdateJobsAgentByTag updates the agent provider and model of pending jobs that match the given tag.
func (o *Orchestrator) UpdateJobsAgentByTag(ctx context.Context, tag string, agentProvider string, agentModel string, logger *slog.Logger) (int, error) {
	o.mu.Lock()
	var jobIDs []string
	lowerTag := strings.ToLower(tag)

	for id, job := range o.pendingJobs {
		for _, t := range job.WorkItem.Tags {
			if strings.ToLower(t) == lowerTag {
				jobIDs = append(jobIDs, id)
				break
			}
		}
	}
	o.mu.Unlock()

	count := 0
	for _, id := range jobIDs {
		if err := o.UpdateJobAgent(ctx, id, agentProvider, agentModel, logger); err == nil {
			count++
		}
	}

	return count, nil
}

// UpdateJobsAgentByMatch updates the agent provider and model of pending jobs that match the given regex.
func (o *Orchestrator) UpdateJobsAgentByMatch(ctx context.Context, match string, agentProvider string, agentModel string, logger *slog.Logger) (int, error) {
	matcher, err := regexp.Compile("(?i)" + match)
	if err != nil {
		return 0, fmt.Errorf("invalid match regex: %w", err)
	}

	o.mu.Lock()
	var jobIDs []string
	for id, job := range o.pendingJobs {
		if matcher.MatchString(job.Summary) || matcher.MatchString(job.Error) {
			jobIDs = append(jobIDs, id)
		}
	}
	o.mu.Unlock()

	count := 0
	for _, id := range jobIDs {
		if err := o.UpdateJobAgent(ctx, id, agentProvider, agentModel, logger); err == nil {
			count++
		}
	}

	return count, nil
}

func (o *Orchestrator) fireJobWebhook(job JobInfo, logger *slog.Logger) {
	payload, err := json.Marshal(job)
	if err != nil {
		if logger != nil {
			logger.Error("Failed to marshal job for webhook", "id", job.ID, "error", err)
		}
		return
	}

	req, err := http.NewRequest(http.MethodPost, job.WorkItem.WebhookURL, bytes.NewBuffer(payload))
	if err != nil {
		if logger != nil {
			logger.Error("Failed to create webhook request", "id", job.ID, "url", job.WorkItem.WebhookURL, "error", err)
		}
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// Set a reasonable timeout for the webhook delivery
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if logger != nil {
			logger.Error("Failed to send job webhook", "id", job.ID, "url", job.WorkItem.WebhookURL, "error", err)
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if logger != nil {
			logger.Info("Job webhook delivered successfully", "id", job.ID, "url", job.WorkItem.WebhookURL, "status", resp.StatusCode)
		}
	} else {
		if logger != nil {
			logger.Warn("Job webhook delivered with non-success status", "id", job.ID, "url", job.WorkItem.WebhookURL, "status", resp.StatusCode)
		}
	}
}

// CancelJobDownstream cancels a job and all its transitive downstream dependencies that are pending or active.
func (o *Orchestrator) CancelJobDownstream(ctx context.Context, jobID string, logger *slog.Logger) ([]string, error) {
	// Need a map of ID -> list of jobs that depend on it
	downstreamMap := make(map[string][]string)

	o.mu.RLock()
	// Check if job exists in active or pending jobs
	var rootJobExists bool
	if _, ok := o.activeJobs[jobID]; ok {
		rootJobExists = true
	} else if _, ok := o.pendingJobs[jobID]; ok {
		rootJobExists = true
	}

	if !rootJobExists {
		o.mu.RUnlock()
		return nil, fmt.Errorf("job %s not found in active or pending queue", jobID)
	}

	// Build adjacency list for downstream dependencies among active and pending jobs
	for _, job := range o.activeJobs {
		for _, dep := range job.WorkItem.DependsOn {
			downstreamMap[dep] = append(downstreamMap[dep], job.ID)
		}
	}
	for _, job := range o.pendingJobs {
		for _, dep := range job.WorkItem.DependsOn {
			downstreamMap[dep] = append(downstreamMap[dep], job.ID)
		}
	}
	o.mu.RUnlock()

	// Find all transitive downstream jobs using BFS
	visited := make(map[string]bool)
	queue := []string{jobID}
	var jobsToCancel []string

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if visited[curr] {
			continue
		}
		visited[curr] = true
		jobsToCancel = append(jobsToCancel, curr)

		if dependents, hasDependents := downstreamMap[curr]; hasDependents {
			queue = append(queue, dependents...)
		}
	}

	// Cancel all found jobs
	var canceledIDs []string
	var lastErr error
	for _, id := range jobsToCancel {
		if logger != nil {
			logger.Info("Canceling job downstream", "id", id, "root", jobID)
		}
		if err := o.CancelJob(ctx, id); err == nil {
			canceledIDs = append(canceledIDs, id)
		} else {
			if logger != nil {
				logger.Error("Failed to cancel job downstream", "id", id, "error", err)
			}
			lastErr = err
		}
	}

	return canceledIDs, lastErr
}
