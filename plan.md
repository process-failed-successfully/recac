The goal is to propose and implement ONE missing, high-value feature.
Since this is the RECAC Orchestrator, it manages agents and has CLI usability.

Looking at the CLI flags:
`--tail-active` tails all currently active jobs in real-time.
However, if a user wants to tail logs for *specific* jobs, they have to use `--logs <job-id>` which isn't a live tail, or wait for them. Wait, `--logs` might stream if it's active. But if there are 10 jobs active, and the user wants to tail only the jobs with a specific tag (e.g., `--tail-tag my-app`), they can't. They have to tail ALL active jobs, which can be super noisy.

Feature Proposal: **Targeted Log Tailing (`--tail-tag` and `--tail-match`)**
This would allow a user to use the log multiplexer to stream logs from multiple active jobs simultaneously, but ONLY those that match a specific tag or a specific regex (summary/error).

Another idea: **Bulk Job Creation from File (`--submit-bulk`)**
We already have `--submit-batch`.

Another idea: **Pause/Resume individual jobs**
We have `--hold-job` and `--unhold-job`.

Another idea: **Job Interactivity (Input Request/Response)**
Probably too complex for 1 feature.

Another idea: **Dashboard Filter by Status/Tag in TUI**
The dashboard currently has a global filter (`/: filter`). Wait, it just searches ID and summary.
```go
			idMatch := strings.Contains(strings.ToLower(job.ID), filterText)
			summaryMatch := strings.Contains(strings.ToLower(job.Summary), filterText)
```
What if we add `--tail-tag` and `--tail-match`? Yes, extending the `tail` functionality to support filters is extremely useful for debugging specific pipelines without the noise of other concurrent jobs.
Wait, let's look at `cmd/orchestrator/tail.go`.

```go
func tailActiveJobs(ctx context.Context, host string) error {
	m := &multiplexer{
		host:   host,
		active: make(map[string]context.CancelFunc),
	}
...
```

Let's modify `tailActiveJobs` to accept `tag` and `match` strings.

```go
func tailActiveJobs(ctx context.Context, host, tag, match string) error {
```

Then in `m.poll(ctx)`:

```go
	url := fmt.Sprintf("%s/jobs", m.host)
    // we can use ?tag=X&match=Y to filter!
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
```

Wait, `GET /jobs` already supports `tag` and `match`!
```go
		q.Set("state", "all")
	if tag != "" {
		q.Set("tag", tag)
	}
	if match != "" {
		q.Set("match", match)
	}
```
Wait, the `GET /jobs` endpoint does support `tag` and `match` query params.

Let's verify `GET /jobs` in `api.go`:
```go
		if tagFilter != "" {
            ...
		}
		matchFilter := r.URL.Query().Get("match")
		if matchFilter != "" {
            ...
        }
```
Yes, it does!

So the feature is **"Targeted Real-time Log Tailing"** (`--tail-tag` and `--tail-match`).
Currently, if you spawn a bunch of jobs, they all run concurrently. `--tail-active` dumps logs from ALL of them. This is very hard to read if there are 20 jobs.
If you only care about jobs related to "frontend", you could run `--tail-tag frontend`.
This significantly improves CLI usability for monitoring.

I will:
1. Add `--tail-tag` and `--tail-match` flags in `cmd/orchestrator/main.go`.
2. Update `tailActiveJobs` in `cmd/orchestrator/tail.go` to accept `tag` and `match`.
3. Update `poll()` in `multiplexer` to append `?tag=` and `?match=` to the `/jobs` URL.
4. Update `main.go` to pass these values.
5. Write/Update tests in `cmd/orchestrator/tail_test.go` to verify this behavior.

Let's double check if there are other commands that might be missing something simple but highly valuable.
- `--retry-failed` already has `--retry-match` and `--retry-tag`.
- `--cancel-all` has `--cancel-tag` and `--cancel-match`.
- `--delete-pending-job` has `--delete-pending-tag` and `--delete-pending-match`.
- `--archive-failed` has `--archive-tag` and `--archive-match`.

So adding `--tail-tag` and `--tail-match` brings the `tail` command to parity with the rest of the CLI's bulk operations!

Let's check `tail_test.go` to see how it's tested.
