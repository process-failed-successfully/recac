## 2024-03-24 - API Key Exposure in URL
**Vulnerability:** The Gemini API key was being passed as a query parameter (`?key=%s`) in the URL.
**Learning:** Passing secrets in URLs is a security risk because URLs are often logged by proxies, servers, and client libraries, leading to potential secret leakage.
**Prevention:** Always use HTTP headers (e.g., `x-goog-api-key`) for passing secrets. This keeps them out of URL logs.

## 2024-05-22 - Path Traversal in Session Management
**Vulnerability:** The `SessionManager` used user-provided session names directly in file paths using `filepath.Join`, allowing path traversal (e.g., `../evil`) to create or overwrite arbitrary files.
**Learning:** `filepath.Join` cleans paths but resolves `..`, meaning it doesn't prevent traversal out of a root if the input contains `..`. Always validate user-provided filenames using `filepath.Base` or similar checks.
**Prevention:** Validate all inputs used in file operations. Ensure `filepath.Base(name) == name` to enforce that the input is a filename, not a path.
