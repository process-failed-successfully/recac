1. **Understand the Goal**: Identify ONE missing but high-value feature or improvement for the `recac` orchestrator.
2. **Analysis**:
   - The orchestrator supports metrics tracking (`job.Metrics` map) and exporting them (`/jobs/export/metrics`, `--export-metrics`).
   - However, the `GET /jobs/export/metrics` API only supports CSV export. Wait, the CLI supports `export-format` ('json' or 'csv') for `export-jobs`, but `export-metrics` is hardcoded to CSV.
   - Another area: The orchestrator supports filtering lists by `tag` or `match`, but what if we want to filter by `agent-model` or `agent-provider`?
   - Let's look at the janitor (`internal/orchestrator/janitor.go`). It cleans up Docker containers and logs. What about a feature to proactively prune pending jobs that have been stuck for too long, or maybe a feature to "bulk update priority" (already exists), or maybe "dry-run" a bulk operation?
   - Let's consider a highly useful operational feature: **Bulk "Wait" for multiple jobs to complete based on tags or regex**. The CLI has `--wait-tag` and `--wait-match`. Wait, it already has `--wait-tag` and `--wait-match`.
   - What about an API endpoint to **get logs for multiple jobs simultaneously**? Or stream logs for all active jobs? `--tail-active` does that.
   - Let's check **Job Dependencies**. You can update dependencies. What if we want to visualize the dependency graph? `--tree` displays dependency tree. Export graph is there.
   - Let's look at **Metrics Aggregation API**. `/analytics` returns `TotalMetrics`. But what if you want to export metrics in JSON format, not just CSV?
   - Look at the `Orchestrator` methods in `internal/orchestrator/orchestrator.go`.
   - How about a new subcommand/flag for **re-running the pipeline of a specific job**?
   - Let's look at the memory constraints/guidelines: "The orchestrator implements a Circuit Breaker pattern ... The orchestrator supports exporting job metrics to a deterministic CSV format ... The orchestrator supports bulk job approval ... The orchestrator supports conditional job execution based on dependency outcomes using the run_condition property ..."
   - A missing feature: **Adding a `/jobs/metrics/aggregate` endpoint** that allows aggregating metrics over a time window or for specific tags, returning them as JSON. Or exposing `export-metrics-format` CLI flag to support JSON and CSV.
   - Wait, `export-metrics` is heavily tied to CSV.
   - How about **Streaming the logs of a job via Server-Sent Events (SSE)**? Currently `/jobs/{id}/logs` returns a simple stream `io.Copy(w, logStream)`. It's not standard SSE. Wait, standard HTTP streaming is fine.
   - What about **Job Tagging**? Can we list all unique tags currently in use across the system? That's missing. An API `GET /tags` that returns a list of all active/historical tags. This would be very useful for autocomplete in the UI/CLI.
   - Let's implement **`GET /tags`** API and `--list-tags` CLI flag to show all unique tags across jobs, with counts.

Let's check if `GET /tags` exists.
