1. **Understand current functionality**: We see `list_jobs`, `status`, `logs`, `inspect`, etc. We have `tui` monitoring. The orchestrator has a `/status` API endpoint.
2. **Identify missing feature**: The orchestrator allows `wait-job` (waiting for a single job to complete), but there isn't a way to easily list pending jobs directly from the CLI. Right now, `list-jobs` fetches `GetActiveJobs()` and displays them. Pending jobs *are* returned by `GetActiveJobs()` (it returns both active and pending), but the output doesn't clearly distinguish them, or maybe it does by Status.
Let's see: `ListJobs` shows "ID", "Summary", "Status", "Duration".

Wait, let's look at what could be added. A `--clean` command to clean the orchestrator? A subcommand to retry all jobs that failed?
Actually, what if we added a `/jobs/clean` endpoint, or a `kill` subcommand?
Wait, the prompt asked for "ONE missing but high-value feature or improvement".
Let's analyze what's missing:
- `wait-job` waits for a single job.
- `submit` allows submitting one job.
- What if we add an endpoint to export the job history as a JSON file, or an endpoint to delete the orchestrator's state (clear all jobs)? Wait, `--clear-history` and `--cancel-all` exist.

Another idea: Healthcheck or ping. The `/healthz` endpoint exists.
What if we add the ability to pause and resume the orchestrator? `--pause` and `--resume` exist.

What about a way to view the orchestrator's logs? No, orchestrator logs are stdout.
What about an endpoint to get the status of the agents?

Wait! A very useful feature for an Orchestrator CLI is a command to **stream logs of multiple jobs**, or **stream orchestrator events**.
Or maybe a **"top" command** for jobs? We have a dashboard (`--monitor`).

How about an endpoint/CLI flag to **update a job's priority**? Wait, `--update-priority` exists.
How about **cloning a job**? `--clone-job` exists.
How about **retry-failed**? `--retry-failed` exists.

What if we add a feature to **gracefully shut down** the orchestrator?
What about **searching/filtering jobs** in `list-jobs`? `list-jobs --filter "status=Failed"`?
Let's check `list-jobs` implementation in `main.go`. It just does `/jobs` and prints.

Let's look at `internal/orchestrator/orchestrator.go`. Is there anything missing in the `GetStatus`?
What if we add **Prometheus metrics**? They are already exported (`metrics-port` exists).

Let's think about `submit`: we can `--submit-batch`, `--submit-url`.
What about **blocking until the orchestrator is idle**? i.e., `--wait-idle`. This is extremely useful for CI/CD when you submit a batch of jobs and want to wait until they are all done.
Currently, `--wait` on `--submit-batch` loops through submitted IDs and does `waitForJob(host, id)`.

Let's look at another idea: **Job dependency management improvements**.
There is `--tree` to view the dependency tree.

What if we add the ability to **pause/resume a specific job**? This might be hard depending on the Spawner. Docker supports `pause`/`unpause`. Kubernetes doesn't easily support pausing a Job.

What about **pruning history**?
`--purge-job` removes a specific job.
`--clear-history` removes all completed/failed jobs.
What if we want to clear jobs older than a certain time? `--prune-history --older-than 24h`?

What about **exporting jobs**? `--export-jobs` exists.
