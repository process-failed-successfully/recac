## 2025-05-22 - [Regex vs Strings for Markdown Parsing]
**Learning:** In this codebase, parsing Markdown blocks with `regexp` (even non-greedy ones) was ~600x slower than using `strings.Index` for large inputs. The `(?s)` flag combined with `.*` likely caused significant backtracking or overhead.
**Action:** Prefer manual string slicing over regex for simple delimiter extraction, especially in utility functions that might process large LLM outputs.
