Plan:
1. Create `cmd/orchestrator/explain.go` which handles the logic for `--explain-job <id>`.
2. Add `--explain-job` flag in `cmd/orchestrator/main.go`.
3. The logic in `explainJob(host, jobID string, provider string, model string)`:
    a) Call `GET /jobs/{id}` to get job metadata (Status, Summary, Error).
    b) Call `GET /jobs/{id}/logs` to get logs.
    c) Take last 1000 lines of logs.
    d) Use `agent.NewAgent(provider, model...)` to get the AI client (using API key from viper/config).
    e) Send a prompt: "You are an expert debugger. The following job failed or had an issue..."
    f) Output the result. It can use `glamour.Render` (which is already present in `go.mod` because the TUI uses `glamour`) to render markdown to terminal beautifully!
4. Write tests for `explain.go` in `cmd/orchestrator/explain_test.go`. We can mock the HTTP server to serve the job info and logs, and mock the `Agent` factory or provide a mock agent type (or just mock the HTTP response for OpenAI/OpenRouter if needed, or there is `agent.NewMockAgent()` available in `internal/agent/mock.go`!).
5. Add pre commit steps

This fulfills all requirements:
1. High-value feature: AI-based debugging of jobs natively integrated into the orchestrator CLI!
2. Strong testing: unit testing the HTTP fetching, string manipulation, and rendering paths.
3. Bold and innovative.
