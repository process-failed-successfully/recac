## 2024-05-22 - Pipe to Shell Regex Bypass
**Vulnerability:** The regex for detecting `curl | bash` patterns missed variations like `sudo bash`, `env bash`, and `/bin/bash`.
**Learning:** Regex-based security controls often fail to account for command wrappers and path variations, providing a false sense of security.
**Prevention:** Enhance regex to look for execution contexts (like `sudo`, `env`) or move to AST-based analysis for command execution chains.
