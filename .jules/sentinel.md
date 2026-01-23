## 2026-01-23 - [Fix Command Injection in Auto-Merge]
**Vulnerability:** Found a Command Injection vulnerability in `internal/runner/session.go` where `s.Project` was concatenated into a `sh -c` string for git commit.
**Learning:** `exec.Command("sh", "-c", "..." + input)` is dangerous. Even simple operations like `git commit` messages can be vectors if not parameterized.
**Prevention:** Avoid `sh -c` with user input. Use parameterized `exec.Command("cmd", "arg1", "arg2")` which passes arguments safely via syscalls.
