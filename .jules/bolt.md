## 2024-10-24 - CallGraph Generation Bottleneck
**Learning:** The `GenerateCallGraph` function was performing O(N) linear scans of all nodes for every call site resolution, resulting in O(M*N) complexity. This became a bottleneck as the codebase grew.
**Action:** Always verify complexity of lookups inside loops. Use auxiliary indices (maps) when repeated lookups are needed.
