## 2026-02-10 - Environment Variable Leak in Local Execution
**Vulnerability:** The `UseLocalAgent` mode in `internal/runner/executor.go` was passing `os.Environ()` directly to `exec.CommandContext`, exposing *all* host environment variables (including unrelated secrets like AWS keys, SSH agents) to the executed process.
**Learning:** When executing untrusted code locally, the host environment must be strictly filtered. Do not assume `os.Environ()` is safe.
**Prevention:** Always use an allowlist for environment variables when spawning subprocesses. Implemented filtering in `internal/runner/executor.go` to match the containerized environment's allowlist.
