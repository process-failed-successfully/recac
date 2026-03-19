Wait, having a global `--history-limit` in the orchestrator that automatically trims `completedJobs` when it exceeds the limit (like popping the oldest ones). This protects the orchestrator from OOMing over weeks of runtime.

Let's look at `internal/orchestrator/orchestrator.go` to see how `completedJobs` is managed.
`o.completedJobs = append(o.completedJobs, job)` in `finalizeJob`.
If I add `HistoryLimit int` to `Orchestrator` (defaulting to 1000).
In `finalizeJob`, after appending, check if `len(o.completedJobs) > o.HistoryLimit`, and if so, slice it (and also maybe remove from Persistence).
This is a good, practical feature, but maybe a bit simple?

Let's look at another idea: **Rate Limiting Job Spawning by Regex or Tag**.
You can specify global max concurrency. But what if you want to say "Max 2 jobs running with tag 'integration_tests'"?
If you have a pool of agents, you might want to throttle specific types of jobs (e.g., jobs hitting an external API limit) while letting others run fully.
So, a `--concurrency-limit-tag integration_tests=2`.
Wait, we already have `ConcurrencyGroup` in `WorkItem`!
"When a job is submitted to the orchestrator with a ConcurrencyGroup and CancelInProgress=false, the orchestrator queues it in pendingJobs and executes the group sequentially (concurrency limit = 1 per group)".
It is hardcoded to 1.

Okay, here is another fantastic idea: **Dry-Run mode for the Pipeline definition**.
You can already do `--dry-run-pipeline`.

What about a **Healthcheck API for specific jobs**?
If a job is running for a long time, the orchestrator just waits. If we could define an `HealthcheckCmd` or `HealthcheckURL` on the `WorkItem`, and if it fails, the orchestrator kills it early!

Wait, let's look at the **File Poller**. It just reads `work_items.json`.
What if we add a **TUI Dashboard feature**? Let's check `internal/tui`.
