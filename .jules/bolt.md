## BOLT'S JOURNAL

## 2026-02-15 - Regex vs Strings for Markdown Parsing
**Learning:** `regexp.FindStringSubmatch` on large LLM outputs (e.g. 50KB+ Markdown) is extremely slow (~365µs) compared to `strings.Index` (~236ns), a 1500x difference. The overhead comes from regex engine execution and allocations for submatches.
**Action:** For simple delimiters like ` ```json ` and ` ``` `, always prefer `strings.Index` and slice manipulation over regex. Avoid regex in hot paths processing potentially large strings.
