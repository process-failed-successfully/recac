## 2024-03-24 - API Key Exposure in URL
**Vulnerability:** The Gemini API key was being passed as a query parameter (`?key=%s`) in the URL.
**Learning:** Passing secrets in URLs is a security risk because URLs are often logged by proxies, servers, and client libraries, leading to potential secret leakage.
**Prevention:** Always use HTTP headers (e.g., `x-goog-api-key`) for passing secrets. This keeps them out of URL logs.

## 2024-05-23 - Bypassable Command Blocklist
**Vulnerability:** The command blacklist for detecting sensitive file access only checked for `cat`, `rm`, etc., but missed common tools like `head`, `tail`, `curl`, and `grep`, allowing potential data exfiltration.
**Learning:** Blocklists for commands are inherently brittle and easy to bypass with alternative tools or shell tricks.
**Prevention:** While a comprehensive sandbox is preferred, a defense-in-depth regex should cover a wide range of standard utilities (`head`, `tail`, `awk`, `sed`, `curl`) and focus on blocking access to sensitive file patterns regardless of the command used where possible.
