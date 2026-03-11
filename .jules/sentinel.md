# Sentinel Learnings

## 2024-05-24 - [Command Injection via shell variable]
**Vulnerability:** Found a shell injection vulnerability in `internal/runner/loop.go` where `s.Project` was being concatenated directly into a `sh -c` execution without sanitization.
**Learning:** Even though `s.Project` might seem like an internal ID, any unsanitized concatenation in a shell command (e.g. `sh -c "git commit -m '" + s.Project + "'`) opens up the possibility of command injection if the variable can be manipulated.
**Prevention:** Use array-based command arguments like `exec.Command("git", "commit", "-m", "...")` instead of concatenating a string and executing it via `sh -c`. This ensures the input is treated as an argument to the process, rather than evaluated by the shell.## 2025-04-10 - [Fix Command Injection in K8s Spawner]
**Vulnerability:** Found a command injection vulnerability in `internal/orchestrator/spawner_k8s.go` where `item.ID` and `item.RepoURL` were directly interpolated into a `sh -c` command string using `fmt.Sprintf` with `%q`.
**Learning:** `fmt.Sprintf` with `%q` only escapes strings using double quotes. However, double-quoted strings in bash do not prevent shell expansions (e.g. `$(malicious_command)` or `` `malicious_command` ``). This allows for easy shell command injection if external user input reaches these variables.
**Prevention:** Instead of string interpolation with `%q`, assemble shell command arguments into a string slice and use `shellquote.Join(agentCmd...)` from `github.com/kballard/go-shellquote` to properly construct and escape commands.
## 2026-03-06 - [Fix Path Traversal in Prompt Loading]
**Vulnerability:** Found a Path Traversal vulnerability (CWE-22) in `internal/agent/prompts/prompts.go` where `GetPrompt(name string, vars map[string]string)` concatenated the `name` parameter with `.md` directly into a `filepath.Join` without sanitization.
**Learning:** Functions like `filepath.Join` resolve path sequences, but do not prevent escaping the intended base directory if the untrusted input contains enough `../` sequences (e.g. `filepath.Join("/var/prompts", "../../../etc/passwd")` resolves to `/etc/passwd`).
**Prevention:** Always sanitize filename or path inputs using `filepath.Base()` when the input is intended to reference a file within a specific directory, ensuring it cannot traverse outside the intended directory.

## 2026-03-08 - [Fix Path Traversal in Snapshot CLI Commands]
**Vulnerability:** Found a Path Traversal vulnerability (CWE-22) in `cmd/recac/snapshot.go` where snapshot names passed via CLI arguments were joined without validation using `filepath.Join`.
**Learning:** Even CLI tools executing locally can be susceptible to path traversal if they manage files in a designated directory but accept path separators in inputs. This could inadvertently overwrite or delete unintended files.
**Prevention:** Introduce validation functions (e.g., `validateSnapshotName`) that assert `filepath.Base(name) == name` to strictly enforce that the input is merely a filename and not a directory path.
## 2024-03-11 - [Credential Leakage in Execution Logs]
**Vulnerability:** Command script and standard output strings were only sanitized (`maskSecrets`) inside the error handling branch. Successful command executions logged unmasked scripts and outputs, leading to potential credential leakage.
**Learning:** Security sanitation/masking must be applied universally to all sensitive data flows, not just as an afterthought during error reporting.
**Prevention:** Mask all secrets early in the pipeline or before any logging/returning of potentially sensitive output data.
