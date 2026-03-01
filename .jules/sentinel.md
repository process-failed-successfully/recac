# Sentinel Learnings

## 2024-05-24 - [Command Injection via shell variable]
**Vulnerability:** Found a shell injection vulnerability in `internal/runner/loop.go` where `s.Project` was being concatenated directly into a `sh -c` execution without sanitization.
**Learning:** Even though `s.Project` might seem like an internal ID, any unsanitized concatenation in a shell command (e.g. `sh -c "git commit -m '" + s.Project + "'`) opens up the possibility of command injection if the variable can be manipulated.
**Prevention:** Use array-based command arguments like `exec.Command("git", "commit", "-m", "...")` instead of concatenating a string and executing it via `sh -c`. This ensures the input is treated as an argument to the process, rather than evaluated by the shell.