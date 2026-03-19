Let's see if there is another feature that could be even better.
What about an `--explain-group` or **"Analyze Failures"** API?
A `POST /jobs/analyze/failures` that uses AI to summarize all recent failures and proposes a global fix.
Wait, that requires calling AI inside the orchestrator logic. The orchestrator already has `explainJob` AI call in `api.go`. So it's very feasible.
But is `timeline` more universally useful without AI keys? Yes. Visualizing orchestration is the primary purpose of an orchestrator CLI.

Let's do **Job Timeline CLI Command**!

## Plan for Job Timeline
1. Add CLI flags: `--timeline` (bool) and `--timeline-limit` (int, default 20) in `cmd/orchestrator/main.go`.
2. Add `viper.BindPFlag` for both.
3. Add `if viper.GetBool("orchestrator.timeline") { printTimeline(...) }` block in `run()`.
4. Create `func printTimeline(host string, limit int)` in `cmd/orchestrator/main.go` (or a separate file like `timeline.go`).
5. `printTimeline` will:
   - Call `GET /jobs?state=all`.
   - Sort by `StartTime` descending. Take the top `limit` jobs.
   - If a job is Pending or StartTime is zero, ignore it or render it differently.
   - Calculate the overall Min `StartTime` and Max `EndTime` (if running, use `time.Now()`).
   - If TotalDuration = Max - Min.
   - For each job, calculate `StartOffset` = `(job.StartTime - Min) / TotalDuration` and `JobLength` = `(job.EndTime - job.StartTime) / TotalDuration`.
   - Render a string of width `W` (e.g. 50 columns): spaces, then blocks `█` based on status color.
6. Write a comprehensive test in `cmd/orchestrator/timeline_test.go` using an `httptest.Server` returning mocked jobs, capture `stdout`, and assert the correct structure and lengths.

Wait, are there any constraints?
"Do not just do 'cleanup' or 'documentation' unless it is critical. Focus on functional value."
Timeline is great functional value.
"Do not break existing tests."
"You MUST add strong testing (unit or integration) to verify it."

Let's double-check the Bubbletea TUI code. Could I add this to the TUI too?
No, building an ANSI text-based timeline for the CLI output is simpler and very robust, and aligns perfectly with `--list-jobs` output format. The TUI is complex and requires full bubbletea model state management. A CLI feature is more direct and useful for scripting or quick glances.

Let's look at `cmd/orchestrator/main.go` to find where to put the code.
