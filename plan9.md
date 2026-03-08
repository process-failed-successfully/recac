Let's consider **Ad-Hoc Agent Interactive Shell**. (Like `docker exec -it ...`).
No, the agent is headless.

How about **Time Travel/Dry-Run Visualization**?
I will add **Auto-Retry** because it is an essential piece of functionality for any robust orchestrator handling transient failure (e.g. Docker daemon restarted, k8s pod evicted, network timeout).

Wait, I saw `GetAnalytics` was an idea. What about `Analytics` endpoint in the API + `--stats` flag? It is simple to implement, extremely useful, and very safe (won't break existing tests unless I break API).

Wait! The prompt wants me to identify ONE missing but high-value feature or improvement.
Let's analyze what's wrong with the orchestrator currently.
If an agent fails due to API rate limit or transient error, the job is marked Failed. The user has to manually run `orchestrator retry-failed` or use the UI to retry it.
If I add `MaxRetries` to the Orchestrator Config, then `spawnWorker` can automatically retry jobs!

Let's look at `internal/orchestrator/orchestrator.go`:
```go
type Orchestrator struct {
	// ...
	MaxRetries    int
	RetryDelay    time.Duration
}
```
If `MaxRetries > 0`, we need a way to track retries per job.
Add `RetryCount int` to `JobInfo` or `WorkItem`.
Since `WorkItem` represents the input, `JobInfo` represents the state.
Let's add `RetryCount int json:"retry_count"` to `JobInfo`.

When `spawnWorker` finishes with `spawnErr != nil`, it checks:
```go
		if spawnErr != nil {
			if o.MaxRetries > 0 && job.RetryCount < o.MaxRetries {
				// Retry
				job.RetryCount++
				job.Status = "Retrying"
				job.Error = spawnErr.Error()
				o.pendingJobs[item.ID] = job
				// Should we delay? We can use time.AfterFunc to process it, or we add RetryAfter and evaluatePendingJobs handles it.
				// Wait! evaluatePendingJobs loops through pendingJobs.
			} else {
				job.Status = "Failed"
				job.Error = spawnErr.Error()
				o.addToHistory(job, logger)
			}
		} else {
            // ...
```

Wait, if we put it in `pendingJobs`, `evaluatePendingJobs` will immediately pick it up if dependencies are met!
If we want to delay it, we must add `RetryAfter time.Time` to `JobInfo`.
```go
type JobInfo struct {
	// ...
	RetryCount  int       `json:"retry_count,omitempty"`
	RetryAfter  time.Time `json:"retry_after,omitempty"`
}
```

In `evaluatePendingJobs`:
```go
	for id, jobInfo := range o.pendingJobs {
		if !jobInfo.RetryAfter.IsZero() && time.Now().Before(jobInfo.RetryAfter) {
			continue // skip this job for now
		}
```
This is a very robust feature.
Let's check if `evaluatePendingJobs` is called frequently enough to pick up the delayed jobs.
It's called in `Run()` loop every `ticker.C` (which is `PollInterval`, e.g., 1 minute).
If `PollInterval` is 1m, and `RetryDelay` is 5s, the job might wait up to 1m.
To fix this, we can just use `time.AfterFunc(retryDelay, func() { o.evaluatePendingJobs(...) })` when scheduling a retry!
This guarantees `evaluatePendingJobs` runs right when the job is ready!

Let's refine the plan for **Auto-Retry**:
1. Add `MaxRetries int` and `RetryDelay time.Duration` to `Orchestrator` struct.
2. Add `RetryCount int` and `RetryAfter time.Time` to `JobInfo`.
3. Modify `spawnWorker` in `orchestrator.go` to handle retries:
   ```go
   if spawnErr != nil && o.MaxRetries > 0 && job.RetryCount < o.MaxRetries && spawnCtx.Err() != context.DeadlineExceeded && spawnCtx.Err() != context.Canceled {
       job.RetryCount++
       job.Status = "Retrying"
       job.Error = spawnErr.Error()
       job.RetryAfter = time.Now().Add(o.RetryDelay)
       o.pendingJobs[item.ID] = job

       // Trigger re-evaluation when delay expires
       time.AfterFunc(o.RetryDelay, func() {
           o.evaluatePendingJobs(context.Background(), logger)
       })

       if logger != nil {
           logger.Info("Job failed, scheduling auto-retry", "id", item.ID, "attempt", job.RetryCount, "max", o.MaxRetries, "delay", o.RetryDelay)
       }
   } else {
       // normal failure handling
       job.Status = "Failed"
       // ...
   }
   ```
4. Modify `evaluatePendingJobs` to check `RetryAfter`:
   ```go
   if !jobInfo.RetryAfter.IsZero() && time.Now().Before(jobInfo.RetryAfter) {
       continue
   }
   ```
5. Add CLI flags `--max-retries` and `--retry-delay` in `cmd/orchestrator/main.go`.
6. Write unit tests for auto-retry behavior.

Wait! If we use `spawnCtx` in `time.AfterFunc(o.RetryDelay, func() { o.evaluatePendingJobs(context.Background(), logger) })`, we must be careful with context! Wait, `evaluatePendingJobs` doesn't take context that gets cancelled. We can pass a background context or a global orchestrator context.

Wait, what if the user cancels the job while it is waiting for retry?
`CancelJob` checks `pendingJobs` and removes it.
```go
func (o *Orchestrator) CancelJob(ctx context.Context, jobID string) error {
	o.mu.Lock()
	if job, exists := o.pendingJobs[jobID]; exists {
		delete(o.pendingJobs, jobID)
		job.Status = "Canceled"
        // ...
		o.addToHistory(job, nil)
		o.mu.Unlock()
		return nil
	}
	o.mu.Unlock()
```
This perfectly works! If it's in `pendingJobs` waiting for retry, `CancelJob` will find it and mark it "Canceled" and remove it. When the `time.AfterFunc` fires, the job won't be in `pendingJobs` anymore!
This is incredibly elegant.

Let's double check `processWorkItem` logic.
Wait, `spawnWorker` gets a copy of `item`. If it puts `job` back into `pendingJobs`, it has `job.WorkItem = item`.
But wait! `spawnWorker` removes it from `activeJobs` and puts it into `history` currently.
Let's see `spawnWorker` defer block:
```go
	defer func() {
		o.mu.Lock()
		// Move to history
		if job, ok := o.activeJobs[item.ID]; ok {
			job.EndTime = time.Now()
			job.ThreadState = threadState
			if spawnErr != nil {
                // Here we inject retry logic!
                if o.MaxRetries > 0 && job.RetryCount < o.MaxRetries && spawnCtx.Err() != context.DeadlineExceeded && spawnCtx.Err() != context.Canceled {
                    job.RetryCount++
                    job.Status = "Retrying"
                    job.Error = spawnErr.Error()
                    job.RetryAfter = time.Now().Add(o.RetryDelay)
                    job.EndTime = time.Time{} // Reset EndTime since it's not done
                    o.pendingJobs[item.ID] = job

                    if logger != nil {
                        logger.Info("Job failed, scheduling auto-retry", "id", item.ID, "attempt", job.RetryCount, "max", o.MaxRetries, "delay", o.RetryDelay)
                    }

                    // Schedule evaluation
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
		}

		o.activeSpawns--
		delete(o.activeJobs, item.ID)
		o.mu.Unlock()
        // ...
	}()
```
Wait, if it's auto-retrying, what happens to `totalSpawns` and `activeSpawns`?
`activeSpawns` is correctly decremented when it leaves `activeJobs` and goes to `pendingJobs`.
When `evaluatePendingJobs` processes it again, it calls `processWorkItem`, which increments `activeSpawns` and `totalSpawns`! This is PERFECT!

What about `evaluatePendingJobs` checking dependencies?
```go
	for id, jobInfo := range o.pendingJobs {
        if !jobInfo.RetryAfter.IsZero() && time.Now().Before(jobInfo.RetryAfter) {
            continue
        }
		item := jobInfo.WorkItem
		met, failedDep := o.checkDependenciesMetLocked(item.DependsOn)
```
Wait, if the job is just retrying, its dependencies were already met. They will still be met! `checkDependenciesMetLocked` will return `true` because the dependency is in `completedJobs`.

Wait, in `processWorkItem`, it creates a NEW `JobInfo`:
```go
	job := JobInfo{
		ID:        item.ID,
		Summary:   item.Summary,
		StartTime: time.Now(),
		Status:    "Spawning",
		WorkItem:  item,
	}
	o.activeJobs[item.ID] = job
```
If we do this, the `RetryCount` and `RetryAfter` fields will be LOST when `processWorkItem` moves it from `pendingJobs` to `activeJobs`!
Ah!
We must pass the `JobInfo` from `pendingJobs` into `activeJobs`, or at least copy `RetryCount`!
```go
	job := JobInfo{
		ID:        item.ID,
		Summary:   item.Summary,
		StartTime: time.Now(),
		Status:    "Spawning",
		WorkItem:  item,
	}
```
If `item.ID` was in `pendingJobs` (e.g. `evaluatePendingJobs` deletes it from `pendingJobs` before calling `processWorkItem`), `processWorkItem` doesn't know it was retrying unless it takes `JobInfo` or we put `RetryCount` in `WorkItem`?
Wait, if `evaluatePendingJobs` calls `processWorkItem(ctx, item, logger)`, `processWorkItem` doesn't know about `JobInfo`.
To fix this, we can make `processWorkItem` look at `pendingJobs`? No, `evaluatePendingJobs` deletes it from `pendingJobs` BEFORE calling `processWorkItem`:
```go
		if met {
			toProcess = append(toProcess, jobInfo) // Pass JobInfo instead of WorkItem
			delete(o.pendingJobs, id)
```
Wait, `toProcess` stores `WorkItem` currently.
If we change `toProcess` to store `JobInfo`? But `processWorkItem` is also called by `Run()` for new items, and `SubmitJob()` for new items!
Instead of changing `processWorkItem` to take `JobInfo`, we can just add `RetryCount` to `WorkItem`!
No, `WorkItem` is the payload from Jira/CLI. It should not track execution state.

Alternatively, `processWorkItem` can accept an optional `existingJob *JobInfo`?
Or better, we don't delete it from `pendingJobs` until `processWorkItem` succeeds? No, `processWorkItem` checks `if _, exists := o.pendingJobs[item.ID]; exists`.
Ah, `processWorkItem` checks `pendingJobs`:
```go
	if _, exists := o.pendingJobs[item.ID]; exists {
		o.mu.Unlock()
		return fmt.Errorf("job %s is already pending dependencies", item.ID)
	}
```
So `evaluatePendingJobs` MUST delete it before calling `processWorkItem`.
If `evaluatePendingJobs` deletes it, it can't pass the `RetryCount` via `pendingJobs`.

Solution:
Change `processWorkItem(ctx context.Context, item WorkItem, logger *slog.Logger)`
to `processWorkItem(ctx context.Context, item WorkItem, retryCount int, logger *slog.Logger)`.
And in `Run` and `SubmitJob`, we pass `0`.
In `evaluatePendingJobs`:
```go
type pendingToProcess struct {
    item WorkItem
    retryCount int
}
```
Then in `processWorkItem`:
```go
	job := JobInfo{
		ID:         item.ID,
		Summary:    item.Summary,
		StartTime:  time.Now(),
		Status:     "Spawning",
		WorkItem:   item,
        RetryCount: retryCount,
	}
```
Yes! This is very clean.
