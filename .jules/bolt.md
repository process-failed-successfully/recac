## BOLT'S JOURNAL

## 2025-05-18 - [Regex Recompilation]
**Learning:** Found `regexp.MustCompile` inside `ParseFileBlocks`, causing recompilation on every call. This is a common anti-pattern but impactful here as it's used in CLI commands parsing LLM output.
**Action:** Always scan for `regexp.MustCompile` inside function bodies during profile phase. Move to package-level variables.
