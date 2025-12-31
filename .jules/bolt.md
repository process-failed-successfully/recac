## 2024-05-23 - [Regex Performance]
**Learning:** Compiling regex inside a function called frequently (or even moderately) is expensive in Go. Pre-compiling to package-level variables reduced execution time by ~50% for matched cases and ~50x for unmatched cases.
**Action:** Always pre-compile regexes in Go using 'var re = regexp.MustCompile(...)' at package level.
