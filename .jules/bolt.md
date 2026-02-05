## 2024-05-23 - [Initial Exploration]
**Learning:** The project is a distributed Go application (`recac`) for autonomous coding. It uses `cobra` for CLI, `bubbletea` for TUI, and integrates with Docker/K8s.
**Action:** Focus on Go-specific performance patterns (regex, string building, I/O buffering) in the `internal/` directory where the core logic resides.

## 2024-05-23 - [Optimization] Accelerating Line Lookup
**Learning:** In code scanners, iterating strings to count newlines for every finding is O(N*M). Precomputing line offsets and using binary search reduces this to O(N + M log N). Also, avoiding `strings.Split` for line-by-line processing reduces allocations.
**Action:** Use `sort.SearchInts` on precomputed offsets for line number lookups in parsers/scanners.
