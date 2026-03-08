import sys

def main():
    with open("internal/orchestrator/orchestrator.go", "r") as f:
        content = f.read()

    # Add Retry fields to JobInfo
    content = content.replace(
        "	WorkItem    WorkItem  `json:\"work_item\"`\n	ThreadState string    `json:\"thread_state,omitempty\"`\n}",
        "	WorkItem    WorkItem  `json:\"work_item\"`\n	ThreadState string    `json:\"thread_state,omitempty\"`\n	RetryCount  int       `json:\"retry_count,omitempty\"`\n	RetryAfter  time.Time `json:\"retry_after,omitempty\"`\n}"
    )

    # Add MaxRetries and RetryDelay to Orchestrator
    content = content.replace(
        "	notifier          Notifier\n}",
        "	notifier          Notifier\n	MaxRetries        int\n	RetryDelay        time.Duration\n}"
    )

    # processWorkItem sig update
    content = content.replace(
        "func (o *Orchestrator) processWorkItem(ctx context.Context, item WorkItem, logger *slog.Logger) error {",
        "func (o *Orchestrator) processWorkItem(ctx context.Context, item WorkItem, retryCount int, logger *slog.Logger) error {"
    )

    # SubmitJob -> processWorkItem
    content = content.replace(
        "func (o *Orchestrator) SubmitJob(ctx context.Context, item WorkItem, logger *slog.Logger) error {\n	return o.processWorkItem(ctx, item, logger)\n}",
        "func (o *Orchestrator) SubmitJob(ctx context.Context, item WorkItem, logger *slog.Logger) error {\n	return o.processWorkItem(ctx, item, 0, logger)\n}"
    )

    # RetryJob -> processWorkItem
    content = content.replace(
        "	logger.Info(\"Retrying job\", \"id\", jobID)\n	return o.processWorkItem(ctx, workItem, logger)",
        "	logger.Info(\"Retrying job\", \"id\", jobID)\n	return o.processWorkItem(ctx, workItem, 0, logger)"
    )

    # RetryFailedJobs -> processWorkItem
    content = content.replace(
        "		if err := o.processWorkItem(ctx, item, logger); err == nil {",
        "		if err := o.processWorkItem(ctx, item, 0, logger); err == nil {"
    )

    # Run -> processWorkItem
    content = content.replace(
        "			if err := o.processWorkItem(ctx, item, logger); err != nil {",
        "			if err := o.processWorkItem(ctx, item, 0, logger); err != nil {"
    )

    # processWorkItem Pending job
    content = content.replace(
        "			job := JobInfo{\n				ID:        item.ID,\n				Summary:   item.Summary,\n				StartTime: time.Now(),\n				Status:    \"Pending\",\n				WorkItem:  item,\n			}",
        "			job := JobInfo{\n				ID:        item.ID,\n				Summary:   item.Summary,\n				StartTime: time.Now(),\n				Status:    \"Pending\",\n				WorkItem:  item,\n				RetryCount: retryCount,\n			}"
    )

    # processWorkItem Spawning job
    content = content.replace(
        "	job := JobInfo{\n		ID:        item.ID,\n		Summary:   item.Summary,\n		StartTime: time.Now(),\n		Status:    \"Spawning\",\n		WorkItem:  item,\n	}",
        "	job := JobInfo{\n		ID:        item.ID,\n		Summary:   item.Summary,\n		StartTime: time.Now(),\n		Status:    \"Spawning\",\n		WorkItem:  item,\n		RetryCount: retryCount,\n	}"
    )

    # evaluatePendingJobs logic
    evaluate_old = """	var toProcess []WorkItem
	for id, jobInfo := range o.pendingJobs {
		item := jobInfo.WorkItem
		met, failedDep := o.checkDependenciesMetLocked(item.DependsOn)
		if met {
			toProcess = append(toProcess, item)
			delete(o.pendingJobs, id)"""

    evaluate_new = """	type pendingJob struct {
		item       WorkItem
		retryCount int
	}
	var toProcess []pendingJob
	for id, jobInfo := range o.pendingJobs {
		if !jobInfo.RetryAfter.IsZero() && time.Now().Before(jobInfo.RetryAfter) {
			continue
		}
		item := jobInfo.WorkItem
		met, failedDep := o.checkDependenciesMetLocked(item.DependsOn)
		if met {
			toProcess = append(toProcess, pendingJob{item: item, retryCount: jobInfo.RetryCount})
			delete(o.pendingJobs, id)"""

    content = content.replace(evaluate_old, evaluate_new)

    sort_old = """	// Sort pending jobs by Priority (descending) and ID (ascending) to ensure stable processing order
	sort.SliceStable(toProcess, func(i, j int) bool {
		if toProcess[i].Priority != toProcess[j].Priority {
			return toProcess[i].Priority > toProcess[j].Priority
		}
		return toProcess[i].ID < toProcess[j].ID
	})

	for _, item := range toProcess {
		if err := o.processWorkItem(ctx, item, logger); err != nil {
			if logger != nil {
				logger.Error(\"Failed to start pending job\", \"id\", item.ID, \"error\", err)
			}
			if err == ErrAtCapacity {
				// Put it back in pendingJobs
				o.mu.Lock()
				o.pendingJobs[item.ID] = JobInfo{
					ID:        item.ID,
					Summary:   item.Summary,
					StartTime: time.Now(),
					Status:    \"Pending\",
					WorkItem:  item,
				}
				o.mu.Unlock()
			}
		}
	}"""

    sort_new = """	// Sort pending jobs by Priority (descending) and ID (ascending) to ensure stable processing order
	sort.SliceStable(toProcess, func(i, j int) bool {
		if toProcess[i].item.Priority != toProcess[j].item.Priority {
			return toProcess[i].item.Priority > toProcess[j].item.Priority
		}
		return toProcess[i].item.ID < toProcess[j].item.ID
	})

	for _, pJob := range toProcess {
		item := pJob.item
		if err := o.processWorkItem(ctx, item, pJob.retryCount, logger); err != nil {
			if logger != nil {
				logger.Error("Failed to start pending job", "id", item.ID, "error", err)
			}
			if err == ErrAtCapacity {
				// Put it back in pendingJobs
				o.mu.Lock()
				o.pendingJobs[item.ID] = JobInfo{
					ID:        item.ID,
					Summary:   item.Summary,
					StartTime: time.Now(),
					Status:    "Pending",
					WorkItem:  item,
					RetryCount: pJob.retryCount,
				}
				o.mu.Unlock()
			}
		}
	}"""

    content = content.replace(sort_old, sort_new)

    # spawnWorker retry handling
    spawn_defer_old = """		// Move to history
		if job, ok := o.activeJobs[item.ID]; ok {
			job.EndTime = time.Now()
			job.ThreadState = threadState
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
		o.mu.Unlock()"""

    spawn_defer_new = """		// Move to history
		if job, ok := o.activeJobs[item.ID]; ok {
			job.ThreadState = threadState
			if spawnErr != nil {
				if o.MaxRetries > 0 && job.RetryCount < o.MaxRetries && spawnCtx.Err() != context.DeadlineExceeded && spawnCtx.Err() != context.Canceled {
					job.RetryCount++
					job.Status = "Retrying"
					job.Error = spawnErr.Error()
					job.RetryAfter = time.Now().Add(o.RetryDelay)
					o.pendingJobs[item.ID] = job

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
				}
			} else {
				job.EndTime = time.Now()
				job.Status = "Completed"
				o.addToHistory(job, logger)
			}
		}

		o.activeSpawns--
		delete(o.activeJobs, item.ID)
		o.mu.Unlock()"""

    content = content.replace(spawn_defer_old, spawn_defer_new)

    with open("internal/orchestrator/orchestrator.go", "w") as f:
        f.write(content)

if __name__ == "__main__":
    main()
