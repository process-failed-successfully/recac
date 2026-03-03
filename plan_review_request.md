I have noticed that the orchestrator API supports pausing, resuming, getting logs, retrying jobs, getting status, getting jobs, submitting jobs, and canceling a single job (`DELETE /jobs/{id}`).
However, it lacks an endpoint and a CLI command to cancel all running jobs at once.

My proposed feature is to implement a `--cancel-all` CLI flag in the orchestrator binary that sends a request to a new `DELETE /jobs` endpoint to cancel all currently active jobs.

Steps:
1.  **Add `DELETE /jobs` API endpoint** in `internal/orchestrator/api.go`. This endpoint will iterate through all active jobs and call `orch.CancelJob(ctx, id)` for each.
2.  **Add `CancelAllJobs` method** to the `Orchestrator` struct in `internal/orchestrator/orchestrator.go` to safely iterate and cancel active jobs.
3.  **Update `cmd/orchestrator/main.go`** to parse the `--cancel-all` flag and call the new cancel-all logic.
4.  **Implement `cancelAllJobs` function** in `cmd/orchestrator/submission.go` (or `main.go`) to make the HTTP DELETE request to `/jobs`.
5.  **Write unit tests** to verify the new endpoint and CLI behavior.
