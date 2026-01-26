## 2025-05-15 - Unprotected Internal Dashboard
**Vulnerability:** The internal dashboard (`internal/web/server.go`) was vulnerable to Slowloris attacks (missing `ReadHeaderTimeout`) and lacked security headers (CSP, etc.), potentially allowing XSS if static content was compromised.
**Learning:** Even internal, localhost-only tools should follow security best practices ("Defense in Depth"). A local dashboard can be a vector if the developer visits a malicious site or if local malware interacts with it.
**Prevention:** Always use `http.Server` with explicit timeouts instead of `http.ListenAndServe`. Always apply basic security headers (`Content-Security-Policy`, `X-Frame-Options`, `X-Content-Type-Options`) even for internal tools.
