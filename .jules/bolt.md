## 2025-12-28 - Regex Optimization in JSON Cleaner
**Learning:** `regexp.MustCompile` inside a function called frequently (like `cleanJSON` in `planner.go`) can cause significant performance overhead due to repeated compilation. Moving it to package-level variables improved performance by ~4x (4674ns -> 1137ns).
**Action:** Always verify if regex compilation is happening in a hot path or loop, and move it to package-level `var` or `init()` if the pattern is constant.
