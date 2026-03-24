## 2025-03-05 - Avoid repeatedly compiling regular expressions
**Learning:** Compiling a regular expression is a relatively expensive operation because it parses the pattern and constructs a state machine. Compiling it inside a function means it gets re-compiled every single time the function is called.
**Action:** Always hoist `regexp.MustCompile` to the package level as a global variable. This ensures it is compiled exactly once when the package is initialized, which is a classic and highly recommended Go performance optimization.

## 2025-03-05 - Avoid multiple string allocations for simple character replacements
**Learning:** Using `strings.ReplaceAll` multiple times or chaining it with `strings.ToUpper` creates intermediate string allocations. Using `regexp.ReplaceAllString` for simple character substitution is also extremely slow compared to raw byte manipulation.
**Action:** For simple string sanitization (like formatting an ID into a valid metric name or environment variable), use a single-pass loop over the string bytes with a pre-sized `strings.Builder` (using `sb.Grow(len(s))`). This eliminates intermediate allocations and is substantially faster.

## 2025-03-05 - Avoid multiple string allocations for simple character replacements
**Learning:** Using `strings.ReplaceAll` multiple times or chaining it with `strings.ToLower` creates intermediate string allocations. Using `regexp.ReplaceAllString` for simple character substitution is also extremely slow compared to raw byte manipulation.
**Action:** For simple string sanitization (like formatting an ID into a valid metric name, environment variable, or pipeline job ID), use a single-pass loop over the string bytes with a pre-sized `strings.Builder` (using `sb.Grow(len(s))`). This eliminates intermediate allocations and is substantially faster.

## 2025-03-05 - Avoid chained string replacements; use fast-path zero-allocation checks
**Learning:** Chaining `strings.ReplaceAll` multiple times creates numerous intermediate string allocations, as each call allocates a brand new string for the entire result even if no changes occur.
**Action:** Always implement a zero-allocation fast-path check first using `strings.IndexByte` to verify if any target characters exist. If none exist, return the original string immediately. If replacements are needed, fall back to a single-pass loop over the string bytes with a pre-allocated `strings.Builder` (`sb.Grow(len(s))`). This guarantees exactly zero or one allocation per string operation, resulting in significant speedups for string formatting utility functions like `sanitizeID` or `safeName`.
## 2024-03-20 - [Redundant checks with strings.ReplaceAll]
**Learning:** `strings.ReplaceAll` internally checks for substring presence before allocating memory.
**Action:** Do not use `strings.Contains` to check if a substring exists before calling `strings.ReplaceAll` in Go, as it results in redundant checks and slower performance.

## 2024-03-21 - [Single pass builder fails with JSON]
**Learning:** When trying to implement a single pass builder using `strings.IndexByte` and `strings.Builder` to prevent multiple string allocations and improve speed, a basic algorithm trying to look for matching `}` after finding a `{` may fail if the string contains a JSON payload. The `{` from the start of the JSON block will be matched with the `}` from the start of the payload block causing everything within to be matched as a key.
**Action:** Always verify if the key matches a value in the `vars` map before blindly writing it out to the builder. If the key does not match any variables in the map, output `{` instead of the original string (since `{` could be the start of a JSON block).

## 2025-05-15 - [Optimize strings.ReplaceAll with strings.NewReplacer]
**Learning:** `strings.NewReplacer` is a standard way to avoid sequential memory allocations and loops compared to chained `strings.ReplaceAll` calls when performing static multi-character replacements. However, its efficiency relies entirely on allocating and building its internal trie structure *once*. If `strings.NewReplacer` is instantiated inline inside a highly-called function, the repeated setup overhead makes it slower than chained `strings.ReplaceAll`.
**Action:** When refactoring chained `strings.ReplaceAll` to `strings.NewReplacer` for performance, always declare the replacer as a package-level global variable (e.g., `var myReplacer = strings.NewReplacer(...)`) so the setup cost is paid only at initialization.

## 2026-03-24 - Critical Path Analysis
**Learning:** Execution time of heavily dependent pipelines is dictated by the critical path of the DAG, not merely the aggregate execution time.
**Action:** Implemented a `--critical-path` command to visualize the longest path utilizing Kahn's topological sort and dynamic programming.
