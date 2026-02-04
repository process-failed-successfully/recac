## 2026-02-04 - Local Agent Directory Escape
**Vulnerability:** The `UseLocalAgent` mode (used when no Docker/K8s) executes bash commands directly on the host without chroot or containerization, allowing `cd ..` to escape the workspace.
**Learning:** Agents running with shell access on the host are inherently risky if command scope isn't strictly enforced. Regex filtering is a first line of defense but not a complete sandbox.
**Prevention:** Added regex-based blocking of `..` and absolute paths for file system commands in the `RegexScanner`. Future improvements should consider `chroot` or dedicated unprivileged users for local execution.
