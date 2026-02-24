## BOLT'S JOURNAL

## 2026-02-18 - Scanner Line Lookup Optimization
**Learning:** Naive linear scanning to find line numbers for every regex match creates an O(N*M) bottleneck, degrading performance significantly on large files.
**Action:** When mapping character offsets to line numbers for multiple matches, pre-calculate newline indices and use binary search (O(log L)) instead of rescanning the string.

## 2026-05-21 - Call Graph Resolution Bottleneck
**Learning:** Naive linear scanning of all nodes to resolve function calls (O(Calls * Nodes)) causes significant performance degradation in large codebases.
**Action:** Use hash map indexes (e.g., map[FuncName][]Node) to reduce lookup complexity to O(Calls * Candidates), making call graph generation scalable.
