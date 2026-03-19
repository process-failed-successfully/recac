Yes, it does. Okay, the UI is quite advanced.

Let's do **Job Wait/Completion Webhooks (`--webhook-url`)** - wait, they already have `webhook-enabled`.

How about **Job Metrics Aggregation**?
If we do `--export-metrics-format=json`, and export metrics nicely.
Wait, let's look at **`--tail-active`**. How does it work? It tails multiple jobs simultaneously.

How about **`GET /jobs/search/logs`** adding support for **Time Range**?
"Show me logs matching 'ERROR' in the last 24 hours".

Let's look at `internal/orchestrator/janitor.go` again. It cleans up Docker containers. But it does NOT clean up completed jobs from `Orchestrator` memory!
If we implement `HistoryLimit` and a background routine (or just inside `Janitor` or `finalizeJob`) to purge old `completedJobs`, this prevents the orchestrator from crashing due to OOM over time. This is genuinely critical for a long-running service.
Wait, what if we also add **Janitor cleanup for old SQLite database records**?
If persistence is enabled (`--db-file`), the SQLite DB stores every job. We need to purge old ones from DB too.
This is a very practical and high-value improvement! "Auto-pruning of historical jobs".

Let's check `internal/orchestrator/persistence.go` to see if there is a way to delete old jobs.
