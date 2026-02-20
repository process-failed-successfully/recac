# Sentinel Journal

## 2026-02-20 - [High] Unauthenticated Orchestrator API
**Vulnerability:** The Orchestrator's HTTP API (port 2112) was completely unauthenticated, allowing any network-adjacent attacker to submit arbitrary jobs, execute code (via agent), and access sensitive job details.
**Learning:** Default "metrics" servers often expand to include control endpoints without adding authentication. `http.ServeMux` makes it easy to add handlers but doesn't enforce auth.
**Prevention:** Implemented an `authMiddleware` that checks for `X-Recac-Token` or `Authorization: Bearer` header. Configured via `RECAC_ORCHESTRATOR_API_KEY`. Always audit "internal" APIs for sensitive operations.
