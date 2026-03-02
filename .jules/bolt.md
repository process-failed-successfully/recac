## 2024-03-24 - File Traversal Performance
**Learning:** `filepath.Walk` is significantly slower than `filepath.WalkDir` due to the overhead of `os.Lstat` calls for every file and directory visited.
**Action:** Always prefer `filepath.WalkDir` when traversing directories in Go to improve performance, especially when there are many files.

## 2026-03-02 - Package-level regex compilation in Go
**Learning:** In Go, calling `regexp.MustCompile` inside a function that is frequently executed (like parsing HTML content in `cleanPageText` inside a loop) can severely degrade performance due to repetitive compilation overhead.
**Action:** Always hoist regex compilations to package-level variables or compile them once during initialization to avoid this unnecessary bottleneck on hot paths.
