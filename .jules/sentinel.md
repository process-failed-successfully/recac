## 2026-02-18 - [Command Injection via Map Keys]
**Vulnerability:** Command injection was possible in `DockerSpawner` because environment variable keys from `WorkItem` were directly concatenated into a shell command string without validation, allowing arbitrary command execution if a key contained shell metacharacters.
**Learning:** When iterating over a map to construct shell commands (e.g., `export key=value`), always validate the KEY as strictly as the value, or use a safe API that handles argument passing without shell interpolation (which `docker exec` does for the command array, but here we were constructing a `sh -c` string).
**Prevention:** Use a strict regex allowlist (e.g., `^[a-zA-Z_][a-zA-Z0-9_]*$`) for any user-controlled input that becomes a variable name in a shell script.
