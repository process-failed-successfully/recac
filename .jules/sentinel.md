## YYYY-MM-DD - [Path Traversal in Artifact API and Cleaner]
**Vulnerability:** Path traversal vulnerability in `internal/orchestrator/api_artifacts.go` allowed directory escape if `jobID` or `filename` was `..`. Also, `runCleanerAgent` lacked secondary validation if `filepath.Rel` returned bypassable paths with mixed slashes.
**Learning:** `filepath.Base()` evaluates exactly `"../"` and `".."` to `".."`. Checking against exactly `.` and `/` is insufficient because an attacker could simply supply `..`. Always add `".."` to the exclusion list when sanitizing inputs with `filepath.Base()`.
**Prevention:** Use `cleanJobID == ".." ` alongside other checks when using `filepath.Base()`, or use Go 1.20's `filepath.IsLocal()` when available. Validate all user-supplied components used in path construction.
## 2025-04-06 - Path Traversal in Filepath.Base
**Vulnerability:** Path Traversal via `filepath.Base` evaluation in `session_manager.go`.
**Learning:** `filepath.Base(name) != name` is insufficient to prevent path traversal because `filepath.Base(".")`, `filepath.Base("..")`, and `filepath.Base("/")` evaluate to themselves, allowing directory manipulation and arbitrary file creation/access when concatenated.
**Prevention:** Explicitly validate dynamic path components against `.`, `..`, and `/` in addition to verifying `filepath.Base(name) == name`.
