## 2026-01-18 - Pipe to Shell Detection
**Vulnerability:** The codebase's security scanner lacked detection for "pipe to shell" patterns (e.g., `curl ... | bash`), which allow execution of untrusted remote code.
**Learning:** Security scanners relying on regex lists must account for command chaining and piping, not just single command keywords.
**Prevention:** Added specific regex `rePipeToShell` to `internal/security/scanner.go` to flag pipes from `curl`/`wget` to interpreters like `bash`/`python`.
