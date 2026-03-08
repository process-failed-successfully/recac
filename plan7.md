If I add `GetJobsFiltered` or just modify `GetJobs`?
Wait, `GetJobs` is used in `LoadHistory`.
If I add a new method `SearchJobs(filter JobFilter) ([]JobInfo, error)`, I have to implement it in all Persistence implementations. There's only `SQLitePersistence` right now and mock ones in tests.

Let's look at another high-value feature, maybe easier to test and highly requested:
**Watch Mode for `list-jobs`**. Like `watch -n 1 list-jobs` but built-in. We can just loop.
Or **Dynamic / Real-time tailing of orchestrator logs**? We have `getLogs` which just dumps them. What if we add `-f` (follow) to `logs`? But `logs` is for a SPECIFIC job, and as discovered, it streams until it ends via `ContainerLogs`! (Wait, `ContainerLogs` returns a stream, and `getLogs` copies it to stdout. So it ALREADY follows).

What about **Bulk Operations**?
`orchestrator cancel --all` exists.
`orchestrator retry-failed --match` exists.

What if we add a **TUI feature**: The ability to cancel, retry, or purge jobs directly from the TUI!
Right now, the TUI dashboard just displays them, maybe clears pending (`P`).
If I can select a job in the TUI, press `c` to cancel it, `r` to retry it, `x` to purge it.
Wait, let's look at `internal/tui/dashboard.go`:
"The `orchestrator` provides a `--clear-history` CLI flag ... an `x` TUI keybinding to selectively purge a specific completed or failed job..." -> memory says `x` exists!
"supports an `e` keybinding to edit/clone an existing job... `s` keybinding to open an interactive form... `o` keybinding... P keybinding..."
The TUI already has a ton of features!

Let's look at the actual prompt:
"Analyze the repository... Identify ONE missing but high-value feature or improvement (e.g., CLI usability, new subcommand, better error handling). Propose and implement this 1 useful feature. You MUST add strong testing (unit or integration) to verify it. Be bold and innovative. Constraints: Do not break existing tests. Focus on functional value."

What if we add a **Job Scheduler (cron) directly into the Orchestrator**?
Wait, `--poller cron` exists! And it takes `--cron-schedule` and `--cron-template`.

What about **Web UI**? The orchestrator is a CLI/daemon. Creating a full Web UI is too big.

What about a **"Top" or "Stats" command**? I liked the Analytics idea.
Let's see if we can do an "Analytics" command that provides a summary of the orchestrator's performance and job history.
Command: `orchestrator --analytics` or `orchestrator --stats`.
Output:
```
Orchestrator Analytics
======================
Total Jobs: 150
Active: 5
Pending: 2
Completed: 120
Failed: 23 (16.08%)

Average Duration (Completed): 2m 34s
Average Duration (Failed): 45s

Top Failing Dependencies:
- SETUP-DB (5 failures)
- BUILD-UI (2 failures)
```
To implement this:
1. Add `GET /stats` API endpoint in `api.go`.
2. Add `GetStats()` method in `Orchestrator` which reads memory (or DB if persistence enabled) to compute these metrics.
3. Update `SQLitePersistence` to add `GetStats()` that queries the database directly with SQL aggregations, or we just fetch `GetJobs(1000)` and calculate in memory for simplicity (since this is SQLite). Given SQLite, we can just do SQL aggregations which is very efficient.
Wait, the Persistence interface would need `GetStats`.

Actually, `GetStatus` already exists, but it only returns:
```go
type Status struct {
	PollInterval      string
	Uptime            string
	LastPoll          time.Time
	LastPollItems     int
	ActiveSpawns      int
	TotalSpawns       int
	Paused            bool
	MaxConcurrentJobs int
}
```
If we augment `Status` to include these stats, or add a separate `Analytics` endpoint. Adding a separate endpoint is cleaner.

Another idea: **Configurable Retry Policies** for Jobs.
Currently, the Orchestrator retries failed jobs via CLI command `--retry-failed`. But what if we want to automatically retry failed jobs?
Like `orchestrator.auto_retry_failed: true` and `orchestrator.max_retries: 3`.
A job fails, orchestrator sees it failed, and automatically resubmits it up to N times.
This is highly requested in any Job orchestrator!
How to track retries?
Add `Retries int` to `JobInfo` or `WorkItem`.
When `spawnWorker` finishes and sets `spawnErr != nil`, if `auto_retry` is enabled, and `job.WorkItem.Retries < MaxRetries`, it increments `Retries`, logs "Auto-retrying job", and re-submits it!
This is incredibly useful, highly functional, and bold.

Let's check if automatic retries exist.
