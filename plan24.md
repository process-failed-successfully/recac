No timeline feature!
"Gantt Chart / Execution Timeline Visualization"
Using `lipgloss` (which is already a dependency!), we can fetch the last N completed/active jobs, scale their durations to terminal width, and print a beautiful color-coded timeline (Gantt chart) in the CLI showing when jobs started, ran, and ended concurrently.
This perfectly answers "visualizing the orchestrator's concurrency and job scheduling".
It's bold, innovative, and high-value!
To implement:
1. Add `--timeline` flag.
2. Add `printTimeline(host string)` func in `cmd/orchestrator/main.go`.
3. Fetch all jobs `GET /jobs?state=all` (maybe filtered by `--timeline-limit N`).
4. Find `min(StartTime)` and `max(EndTime)` to establish the time window.
5. Create a `lipgloss` rendering loop that draws blocks for each job proportionally.
- Colors: Green for Completed, Red for Failed, Yellow for Running, Gray for Pending.

Let's check if we can do this!
I'll write `printTimeline(host string, limit int)` in `cmd/orchestrator/main.go` and add the flag `--timeline` and `--timeline-limit`.
I need a way to test this! The prompt requires strong testing (unit or integration).
So I'll add `func TestPrintTimeline(t *testing.T)` in `cmd/orchestrator/submission_test.go` or `timeline_test.go` that spins up the mock server, feeds it jobs with specific start/end times, captures stdout, and asserts the output contains expected elements.

Let's review the required tools:
We already import `github.com/charmbracelet/lipgloss`.
We can calculate the terminal width (or default to 80 chars for the bar part).
Let's refine the plan.
