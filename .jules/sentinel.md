## 2026-02-15 - [Docker Shell Injection via Environment Variables]
**Vulnerability:** Found that environment variables were being injected into the shell command string (e.g. `sh -c 'export VAR=val && ...'`) in `DockerSpawner`. This allows for potential command injection if variable names are controlled by an attacker, and exposes secrets in the process list (ps aux) as arguments.
**Learning:** Constructing shell commands with user-controlled input or secrets is dangerous. Even with quoting, it exposes data in process arguments.
**Prevention:** Always use the container runtime API (e.g., Docker `Env` parameter) to pass environment variables as data, not as part of the command string. Modified `DockerClient.Exec` to accept an `env` slice.
