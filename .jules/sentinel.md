# Sentinel Learnings

## 2024-05-24 - [Command Injection via shell variable]
**Vulnerability:** Found a shell injection vulnerability in `internal/runner/loop.go` where `s.Project` was being concatenated directly into a `sh -c` execution without sanitization.
**Learning:** Even though `s.Project` might seem like an internal ID, any unsanitized concatenation in a shell command (e.g. `sh -c "git commit -m '" + s.Project + "'`) opens up the possibility of command injection if the variable can be manipulated.
**Prevention:** Use array-based command arguments like `exec.Command("git", "commit", "-m", "...")` instead of concatenating a string and executing it via `sh -c`. This ensures the input is treated as an argument to the process, rather than evaluated by the shell.## 2025-04-10 - [Fix Command Injection in K8s Spawner]
**Vulnerability:** Found a command injection vulnerability in `internal/orchestrator/spawner_k8s.go` where `item.ID` and `item.RepoURL` were directly interpolated into a `sh -c` command string using `fmt.Sprintf` with `%q`.
**Learning:** `fmt.Sprintf` with `%q` only escapes strings using double quotes. However, double-quoted strings in bash do not prevent shell expansions (e.g. `$(malicious_command)` or `` `malicious_command` ``). This allows for easy shell command injection if external user input reaches these variables.
**Prevention:** Instead of string interpolation with `%q`, assemble shell command arguments into a string slice and use `shellquote.Join(agentCmd...)` from `github.com/kballard/go-shellquote` to properly construct and escape commands.
