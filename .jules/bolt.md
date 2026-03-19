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
