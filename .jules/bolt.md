## 2025-03-05 - Avoid repeatedly compiling regular expressions
**Learning:** Compiling a regular expression is a relatively expensive operation because it parses the pattern and constructs a state machine. Compiling it inside a function means it gets re-compiled every single time the function is called.
**Action:** Always hoist `regexp.MustCompile` to the package level as a global variable. This ensures it is compiled exactly once when the package is initialized, which is a classic and highly recommended Go performance optimization.

## 2025-03-05 - Avoid multiple string allocations for simple character replacements
**Learning:** Using `strings.ReplaceAll` multiple times or chaining it with `strings.ToUpper` creates intermediate string allocations. Using `regexp.ReplaceAllString` for simple character substitution is also extremely slow compared to raw byte manipulation.
**Action:** For simple string sanitization (like formatting an ID into a valid metric name or environment variable), use a single-pass loop over the string bytes with a pre-sized `strings.Builder` (using `sb.Grow(len(s))`). This eliminates intermediate allocations and is substantially faster.

## 2025-03-05 - Avoid multiple string allocations for simple character replacements
**Learning:** Using `strings.ReplaceAll` multiple times or chaining it with `strings.ToLower` creates intermediate string allocations. Using `regexp.ReplaceAllString` for simple character substitution is also extremely slow compared to raw byte manipulation.
**Action:** For simple string sanitization (like formatting an ID into a valid metric name, environment variable, or pipeline job ID), use a single-pass loop over the string bytes with a pre-sized `strings.Builder` (using `sb.Grow(len(s))`). This eliminates intermediate allocations and is substantially faster.
