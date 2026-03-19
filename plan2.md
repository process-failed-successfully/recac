Another great idea:
**Configurable "dry-run" mode for bulk operations** (like cancel, purge, clone, hold, unhold, approve, update dependencies, update tags).
Currently, if you run `--cancel-match ".*"` it just nukes everything. A `--dry-run-bulk` flag could return the list of jobs that *would* be affected without actually doing it.
Wait, that requires modifying almost every bulk API.

What about **Bulk Restart/Retry by Match or Tag**? `--retry-match` and `--retry-tag` exist for failed jobs (`--retry-failed`). Does it allow retrying completed or cancelled jobs? `--retry-job` is for a single job.

Let's look at **Job Search API**. `GET /jobs/search/logs` exists. Is there a way to **filter jobs by Environment Variable**?
E.g., "Find all jobs where `DEPLOY_ENV=production`".
Currently, `GET /jobs` has `tag`, `match`, `status` filters. Adding `env_key` and `env_value` query parameters to `GET /jobs` would be extremely powerful.

Let's check `GET /jobs` in `internal/orchestrator/api.go`.
