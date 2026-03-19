Okay, I see there's a huge TUI implementation.

Let's go back to an idea that adds high value: **Job Retries with Exponential Backoff**.
Right now:
```go
// orchestrator.go
orch.RetryDelay = viper.GetDuration("orchestrator.retry_delay")
```
It retries after a fixed delay. If an external service is down, the orchestrator retries `max_retries` times with a 5-second delay. That means it might burn through 5 retries in 25 seconds and fail the job permanently!
If we add **Exponential Backoff for Retries**:
- `WorkItem` can have `RetryBackoff` (bool or strategy name) or we make it global/configurable.
- `Orchestrator` can have `--retry-backoff` flag. If true, the delay doubles each time.
- If Job A fails on attempt 1, retry delay = 5s. Attempt 2 fails, delay = 10s. Attempt 3 fails, delay = 20s.
- This is a standard and crucial feature for any robust orchestrator.

Let's check how retries are currently scheduled.
