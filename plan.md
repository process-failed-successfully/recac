There is no `--stream-events` or `--watch-events` flag in the CLI to tail the orchestrator events. Let's create one.

Adding a `--stream-events` flag to `main.go`. This flag will connect to the `GET /events` SSE (Server-Sent Events) endpoint and print the JSON payload nicely in real-time. This provides immediate observability directly from the terminal without having to launch the TUI or curl the raw endpoint and parse it manually.

Steps to Implement:
1.  **Add the CLI Flag**: In `cmd/orchestrator/main.go`, register the `--stream-events` bool flag.
2.  **Bind and Evaluate the Flag**: Add logic to check if `viper.GetBool("orchestrator.stream_events")` is true. If so, call a new function `streamEvents(ctx, host)` and then return.
3.  **Implement `streamEvents`**:
    - Connect to `${host}/events` via `http.Get`.
    - Check if the status code is 200.
    - Read the response stream using `bufio.Scanner` (or custom reader for SSE). The SSE lines look like `data: {"event": "connected", ...}\n\n`.
    - Parse the JSON payload and format it nicely using `lipgloss`.
      * E.g., `[03:45:12] [job_completed] Job: TASK-123 | Status: Completed | Duration: ...`
    - Gracefully handle `ctx.Done()` or connection drops.
4.  **Write Tests**: Add a unit test `TestStreamEvents` in a new file `cmd/orchestrator/stream_events_test.go` using a mock HTTP server that streams a few dummy events and verifies the formatted output.
