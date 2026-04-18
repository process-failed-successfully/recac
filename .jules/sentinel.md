## YYYY-MM-DD - [Path Traversal in Artifact API and Cleaner]
**Vulnerability:** Path traversal vulnerability in `internal/orchestrator/api_artifacts.go` allowed directory escape if `jobID` or `filename` was `..`. Also, `runCleanerAgent` lacked secondary validation if `filepath.Rel` returned bypassable paths with mixed slashes.
**Learning:** `filepath.Base()` evaluates exactly `"../"` and `".."` to `".."`. Checking against exactly `.` and `/` is insufficient because an attacker could simply supply `..`. Always add `".."` to the exclusion list when sanitizing inputs with `filepath.Base()`.
**Prevention:** Use `cleanJobID == ".." ` alongside other checks when using `filepath.Base()`, or use Go 1.20's `filepath.IsLocal()` when available. Validate all user-supplied components used in path construction.

## 2025-04-06 - Path Traversal in Filepath.Base
**Vulnerability:** Path Traversal via `filepath.Base` evaluation in `session_manager.go`.
**Learning:** `filepath.Base(name) != name` is insufficient to prevent path traversal because `filepath.Base(".")`, `filepath.Base("..")`, and `filepath.Base("/")` evaluate to themselves, allowing directory manipulation and arbitrary file creation/access when concatenated.
**Prevention:** Explicitly validate dynamic path components against `.`, `..`, and `/` in addition to verifying `filepath.Base(name) == name`.

## 2025-04-07 - Path Traversal in Cleaner Agent and Session Manager Unarchive
**Vulnerability:** Path traversal vulnerability in `runCleanerAgent` due to custom logic handling mixed slashes, and `UnarchiveSession` failing to call `validateSessionName`.
**Learning:** Checking for prefix `..` on the result of `filepath.Clean` is not enough to prevent directory escape, because mixed absolute paths or drive letters on Windows can trick `filepath.Rel` and custom string manipulation. Also, functions that build filenames directly must ensure inputs are validated.
**Prevention:** Use Go 1.20's `filepath.IsLocal()` consistently to reject any path traversal strings before passing them to file system functions, while ensuring you accommodate legitimate local paths like `.` if necessary.

## 2024-04-10 - Fixed Path Traversal with `filepath.Base`
**Vulnerability:** Path traversal vulnerability in artifact handling and session manager due to insufficient validation using `filepath.Base` alongside `net/http` `r.PathValue`.
**Learning:** `filepath.Base` extraction on its own is not robust enough against path traversal edge cases. A path like `"a/b"` gets extracted to `"b"`, meaning the ID equality check fails, but the API may process paths maliciously depending on how `r.PathValue` parses slashes.
**Prevention:** Use Go 1.20's `filepath.IsLocal` to handle OS-specific path traversal edge cases (like Windows reserved filenames), and use `filepath.Base(id) == id` to ensure inputs contain no directories.

## 2026-04-13 - Path Traversal Fix in Prompt Loading
**Vulnerability:** Path traversal in `GetPrompt` due to reliance on `filepath.Base()`.
**Learning:** `filepath.Base()` is insufficient to prevent path traversal. It doesn't perform equality checks for input paths, meaning malicious input like `a/b` can bypass security.
**Prevention:** Use a combination of `filepath.IsLocal()`, explicit checks for `.` and `..`, and `filepath.Base(name) == name` to comprehensively validate filenames and prevent path traversal.

## 2026-04-14 - Path Traversal Fix in Undo Manager
**Vulnerability:** Path traversal in `internal/undo/manager.go` via reliance on `filepath.Base()` for backups.
**Learning:** `filepath.Base()` is insufficient to prevent path traversal on its own. `filepath.Base("..")` returns `..`. When appended to a directory path, it can escape the intended directory.
**Prevention:** Use a combination of `filepath.IsLocal()`, explicit checks for `.` and `..`, and `filepath.Base(name) == name` to comprehensively validate filenames and prevent path traversal.
