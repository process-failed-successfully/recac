Let's see if there are any conflicts.
In `evaluatePendingJobs`:
```go
	var toProcess []struct{
        item WorkItem
        retryCount int
    }
	for id, jobInfo := range o.pendingJobs {
        if !jobInfo.RetryAfter.IsZero() && time.Now().Before(jobInfo.RetryAfter) {
            continue
        }
		item := jobInfo.WorkItem
		met, failedDep := o.checkDependenciesMetLocked(item.DependsOn)
		if met {
			toProcess = append(toProcess, struct{
                item WorkItem
                retryCount int
            }{item, jobInfo.RetryCount})
			delete(o.pendingJobs, id)
```

In `processWorkItem`:
```go
func (o *Orchestrator) processWorkItem(ctx context.Context, item WorkItem, retryCount int, logger *slog.Logger) error {
// ...
			// Valid dependencies but not yet completed
			job := JobInfo{
				ID:        item.ID,
				Summary:   item.Summary,
				StartTime: time.Now(),
				Status:    "Pending",
				WorkItem:  item,
                RetryCount: retryCount,
			}
// ...
	job := JobInfo{
		ID:        item.ID,
		Summary:   item.Summary,
		StartTime: time.Now(),
		Status:    "Spawning",
		WorkItem:  item,
        RetryCount: retryCount,
	}
```
Wait, if `processWorkItem` adds it back to `pendingJobs` (e.g. `ErrAtCapacity`), it must also preserve `RetryCount`!
```go
			if err == ErrAtCapacity {
				// Put it back in pendingJobs
				o.mu.Lock()
				o.pendingJobs[item.ID] = JobInfo{
					ID:        item.ID,
					Summary:   item.Summary,
					StartTime: time.Now(),
					Status:    "Pending",
					WorkItem:  item,
                    RetryCount: p.retryCount,
				}
				o.mu.Unlock()
			}
```

This ensures `RetryCount` is carried over correctly.
What about `spawnWorker` handling the retry logic?
```go
	defer func() {
		o.mu.Lock()
		// Move to history
		if job, ok := o.activeJobs[item.ID]; ok {
			job.EndTime = time.Now()
			job.ThreadState = threadState
			if spawnErr != nil {
                if o.MaxRetries > 0 && job.RetryCount < o.MaxRetries && spawnCtx.Err() != context.DeadlineExceeded && spawnCtx.Err() != context.Canceled {
                    job.RetryCount++
                    job.Status = "Retrying"
                    job.Error = spawnErr.Error()
                    job.RetryAfter = time.Now().Add(o.RetryDelay)
                    job.EndTime = time.Time{} // reset end time
                    o.pendingJobs[item.ID] = job

                    if logger != nil {
                        logger.Info("Job failed, scheduling auto-retry", "id", item.ID, "attempt", job.RetryCount, "max", o.MaxRetries, "delay", o.RetryDelay)
                    }

                    // Trigger re-evaluation when delay expires
                    time.AfterFunc(o.RetryDelay, func() {
                        o.evaluatePendingJobs(context.Background(), logger)
                    })
                } else {
                    job.Status = "Failed"
                    job.Error = spawnErr.Error()
                    o.addToHistory(job, logger)
                }
			} else {
				job.Status = "Completed"
                o.addToHistory(job, logger)
			}
```
Wait, `spawnWorker` takes `item WorkItem`, so `job.RetryCount` is inside `JobInfo` map! Which is perfect!

Let's look at `cmd/orchestrator/main.go` and add the CLI flags:
```go
	pflag.Int("max-retries", 0, "Maximum number of automatic retries for failed jobs")
	pflag.Duration("retry-delay", 5*time.Second, "Delay between automatic retries")
```
Bind them to viper:
```go
	viper.BindPFlag("orchestrator.max_retries", pflag.Lookup("max-retries"))
	viper.BindPFlag("orchestrator.retry_delay", pflag.Lookup("retry-delay"))
    // Environment variables
	viper.BindEnv("orchestrator.max_retries", "RECAC_MAX_RETRIES")
	viper.BindEnv("orchestrator.retry_delay", "RECAC_RETRY_DELAY")
```
Set them on the Orchestrator instance:
```go
	orch.MaxRetries = viper.GetInt("orchestrator.max_retries")
	orch.RetryDelay = viper.GetDuration("orchestrator.retry_delay")
```

I'll write tests for this! This is a super strong functional feature.
`orchestrator_autoretry_test.go`
1. Mock spawner that fails 2 times and succeeds the 3rd time.
2. Submit job.
3. Wait for it to become "Completed".
4. Assert that `GetJob` shows `RetryCount == 2`.

Let's check if there are any other `processWorkItem` callers.
`Run` -> `processWorkItem(ctx, item, 0, logger)`
`SubmitJob` -> `processWorkItem(ctx, item, 0, logger)`
`RetryJob` -> `processWorkItem(ctx, item, 0, logger)` (Manual retry resets auto-retry count!)
`RetryFailedJobs` -> `processWorkItem(ctx, item, 0, logger)` (Manual retry resets auto-retry count!)
`cloneJob` calls `SubmitJob`, so it resets.
This is logically completely sound!

I'll start coding this!
