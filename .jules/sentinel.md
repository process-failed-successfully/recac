## 2026-01-30 - [Secure Command Execution]
**Vulnerability:** The `recac heal` command executed user-supplied strings via `sh -c` without timeouts or validation, posing DoS and accidental damage risks.
**Learning:** CLI tools that execute arbitrary commands are inherently risky. While flexibility is a feature, guardrails (timeouts, blocklists) are essential for defense-in-depth, even for developer tools.
**Prevention:** Always use `exec.CommandContext` for external processes. Validate input against known dangerous patterns where feasible.
