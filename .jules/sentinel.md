## 2024-03-24 - API Key Exposure in URL
**Vulnerability:** The Gemini API key was being passed as a query parameter (`?key=%s`) in the URL.
**Learning:** Passing secrets in URLs is a security risk because URLs are often logged by proxies, servers, and client libraries, leading to potential secret leakage.
**Prevention:** Always use HTTP headers (e.g., `x-goog-api-key`) for passing secrets. This keeps them out of URL logs.
