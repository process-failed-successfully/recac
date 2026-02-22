## 2025-05-15 - Environment Variable Leakage in Local Execution

**Vulnerability:** The `executeCommandBlock` function in `internal/runner/executor.go` was propagating all environment variables (`os.Environ()`) to locally executed agent commands. This meant that any secrets (API keys, tokens) present in the Orchestrator's environment were accessible to the agent and could be leaked if the agent executed `env` or similar commands.

**Learning:** When using `os/exec` to run untrusted or semi-trusted code (like LLM-generated scripts), defaulting to `os.Environ()` is dangerous because it breaks isolation. The parent process often holds privileged secrets that the child process does not need and should not see.

**Prevention:** Always use an explicit whitelist of allowed environment variables when spawning child processes. Do not use `os.Environ()` or `cmd.Env = nil` (which defaults to `os.Environ()` in some contexts, though in Go `nil` implies parent env). Explicitly construct the `Env` slice with only the necessary variables (e.g., `PATH`, `HOME`, `LANG`).
