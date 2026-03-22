## 2025-03-22 - [Timing Attack in Webhook Validation]
**Vulnerability:** Trello webhook handler (`POST /webhook/trello`) compared HMAC-SHA1 signatures using basic string equality (`signature != expectedMAC`).
**Learning:** While other webhook endpoints (Jira, GitHub, Linear, GitLab) in `api.go` correctly utilized `hmac.Equal`, the Trello implementation missed this best practice, introducing a potential timing attack vulnerability where an attacker could theoretically infer the valid MAC byte-by-byte based on the comparison time.
**Prevention:** Always verify that cryptographic signatures and MAC comparisons across *all* implemented endpoints use constant-time functions (like `hmac.Equal`) to defend against timing attacks.
