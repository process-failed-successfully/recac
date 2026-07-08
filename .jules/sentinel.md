## 2024-05-24 - Enable CLI bulk priority update
**Vulnerability:**
None (Enhancement).

**Learning:**
The updateBulkPriority function existed but was never invoked in main.go, rendering the --update-priority-tag, --update-priority-match, and --update-priority-group CLI flags non-functional despite being parsed and validated. Added the necessary dispatch logic in main.go.

**Prevention:**
Ensure CLI features are wired end-to-end and test all command branches, not just single-item actions.

## 2024-05-20 - [Hardcoded Database Password in Deployment Scripts]
**Vulnerability:** Found hardcoded `PGPASSWORD=changeit` used in `kubectl exec` commands when deploying the Helm chart and testing the project.
**Learning:** Even in testing or internal deployment scripts, hardcoding credentials inside code risks accidental leakage if the repo is made public or reused as a template. Additionally, executing shell commands with secrets directly in string templates makes the secret visible in process lists and potential logs.
**Prevention:** Always use environment variables (`os.Getenv("POSTGRES_PASSWORD")`) with safe fallbacks for deployments. Be careful about interpolating secrets into command strings; prefer passing secrets securely.

## 2024-06-29 - Add HTTP security headers to web server
**Vulnerability:** Missing HTTP security headers (X-Frame-Options, X-Content-Type-Options, CSP) on the Orchestrator web UI.
**Learning:** Even internal/local-only web UIs should implement defense-in-depth measures like security headers to protect against Clickjacking and MIME-type sniffing.
**Prevention:** Wrap HTTP multiplexers in a security middleware by default to enforce secure headers across all routes.
## 2025-02-26 - [Improve API Key Retrieval Fallback]
**Vulnerability:** The API key fallback in `internal/cmdutils/factory.go` did not check the `secrets.api_key` path in viper before falling back to environment variables.
**Learning:** For defense-in-depth, sensitive credentials (like API keys) should first attempt to be retrieved from the nested `secrets` structure in `viper` (e.g., `viper.GetString("secrets.api_key")`) before relying on top-level configuration values or environment variables. This pattern was established in `internal/orchestrator/api.go` but was missing in the command utility factory.
**Prevention:** Ensure new implementations retrieving sensitive configurations always check the established nested secret structure to provide administrators a unified, secure way of supplying credentials.
