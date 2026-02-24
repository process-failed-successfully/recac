## 2026-02-24 - Insecure Default Configuration Permissions
**Vulnerability:** The default `config.yaml` file was created with world-readable permissions (0644) by `viper.SafeWriteConfig`, potentially exposing sensitive API keys and credentials.
**Learning:** Libraries like `viper` often default to standard file permissions (0644/0666) when creating configuration files, prioritizing ease of access over security for sensitive data.
**Prevention:** Explicitly create configuration files with restrictive permissions (0600) using `os.OpenFile` before allowing libraries to write to them, or configure the library to use strict permissions if supported.
