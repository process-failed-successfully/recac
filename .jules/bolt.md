## 2024-05-23 - [Regex Performance]
**Learning:** Compiling regex inside a function called frequently (or even moderately) is expensive in Go. Pre-compiling to package-level variables reduced execution time by ~50% for matched cases and ~50x for unmatched cases.
**Action:** Always pre-compile regexes in Go using 'var re = regexp.MustCompile(...)' at package level.

## 2025-05-15 - [Defer inside Loops]
**Learning:** Using `defer` inside a loop (e.g., `defer file.Close()`) defers execution until the *function* returns, not the loop iteration. This causes resource leaks (e.g., file descriptors) that persist until the loop finishes.
**Action:** Wrap the loop body in a closure (IIFE) if using `defer`, or explicitly close resources.
