## 2026-01-17 - Local Mode RCE via init.sh
**Vulnerability:** The `recac` agent unconditionally executes `init.sh` from the workspace on startup. In "Local Mode" (when Docker is unavailable or configured for local execution), this script runs directly on the host machine.
**Learning:** Autonomous agents that execute code from a workspace must strictly validate the execution environment. "Local Mode" fallbacks can inadvertently bridge the gap between untrusted code and the host system.
**Prevention:** Explicitly check for containerization indicators (/.dockerenv, KUBERNETES_SERVICE_HOST) before executing lifecycle scripts. Default to "deny" for unverified environments.
