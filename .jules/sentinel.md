## 2025-03-22 - [Timing Attack in Webhook Validation]
**Vulnerability:** Trello webhook handler (`POST /webhook/trello`) compared HMAC-SHA1 signatures using basic string equality (`signature != expectedMAC`).
**Learning:** While other webhook endpoints (Jira, GitHub, Linear, GitLab) in `api.go` correctly utilized `hmac.Equal`, the Trello implementation missed this best practice, introducing a potential timing attack vulnerability where an attacker could theoretically infer the valid MAC byte-by-byte based on the comparison time.
**Prevention:** Always verify that cryptographic signatures and MAC comparisons across *all* implemented endpoints use constant-time functions (like `hmac.Equal`) to defend against timing attacks.
## $(date +%Y-%m-%d) - SQL Injection in Database Cleanup
**Vulnerability:** String concatenation was used with `fmt.Sprintf` to build `DELETE` queries for the DB cleanup routines in both Postgres and SQLite stores.
**Learning:** Even if the string value is hardcoded initially (e.g., `criticalSignals`), passing it through `fmt.Sprintf` directly into `db.Exec` violates security best practices and can become an active vulnerability if the value ever becomes user-configurable or dynamic in the future.
**Prevention:** Always use parameterized SQL queries (e.g., `db.Exec("... IN (?, ?, ?)", val1, val2, val3)` for SQLite, and `$1, $2, $3` for Postgres) even for internal hardcoded lists.
