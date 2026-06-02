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
