## 2026-01-27 - Command Injection in Git Commit Logic
**Vulnerability:** Command injection vulnerability in `internal/runner/loop.go` where `s.Project` (potentially user-controlled) was concatenated into a `sh -c` string for `git commit` message.
**Learning:** The code used `exec.Command("sh", "-c", ...)` to chain git commands (`add && commit`), which is risky when arguments are dynamic. Go's `exec.Command` is safe only when arguments are passed individually, not as a shell string.
**Prevention:** Use `git.Client` which abstracts git operations and uses `exec.Command` safely with separate arguments, or use `exec.Command("git", ...)` directly without shell. Avoid `sh -c` unless absolutely necessary and ensure all inputs are sanitized (or passed as environment/arguments if possible).
