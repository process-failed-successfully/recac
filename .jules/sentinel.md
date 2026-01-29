## 2026-01-29 - [Path Traversal in Command Execution]
**Vulnerability:** The `ProcessResponse` function in `internal/runner/executor.go` executed commands directly without verifying path traversal (e.g., `rm -rf ../file`) when `UseLocalAgent` was true. This bypassed the security scanner which was only run on the full response text in a separate flow.
**Learning:** Checking the full response text is insufficient because complex outputs can mask dangerous commands, and execution points must have their own "last mile" validation.
**Prevention:** Always validate extracted command blocks immediately before execution using a dedicated security scanner, and explicitly block `..` path traversal in sensitive file operations.
