## 2025-02-17 - [Static Analysis Complexity]
**Learning:** The `GenerateCallGraph` function in `internal/analysis` used a naive O(N) lookup for every function call encountered, resulting in O(Calls * Nodes) complexity. This scales poorly (O(N^2)) for large codebases.
**Action:** Always pre-index AST nodes by package and name before traversing the tree for resolution. Use maps for O(1) lookups instead of linear scans.
