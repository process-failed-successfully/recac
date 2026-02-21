## 2025-05-18 - Missing Orchestrator Authentication
**Vulnerability:** The orchestrator API was exposing sensitive endpoints (`/jobs`, `/status`, etc.) without any authentication, allowing unauthenticated job submission and inspection of secrets.
**Learning:** The project relied on an assumption that the orchestrator would be deployed in a secure environment or that `authMiddleware` existed (as hinted in documentation/memory) when it did not.
**Prevention:** Always verify security controls (like authentication middleware) are actually implemented in code, not just described in documentation. Use automated security scans or integration tests that specifically check for unauthenticated access.
