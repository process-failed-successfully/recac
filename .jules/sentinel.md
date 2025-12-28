## 2024-05-23 - Git Branch Name Input Validation
**Vulnerability:** Potential for Git Argument Injection via unvalidated branch names.
**Learning:** Although `os/exec` prevents shell command injection (e.g. `; rm -rf`), it does not prevent argument injection if the input starts with `-` (e.g. `-t` or `--help`).
**Prevention:** Always validate user-controlled input against a strict whitelist (e.g. `^[a-zA-Z0-9._/-]+$`) before passing it to external commands, even when using safe execution wrappers like `exec.Command`.
