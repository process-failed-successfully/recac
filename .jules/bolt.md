## 2024-03-24 - File Traversal Performance
**Learning:** `filepath.Walk` is significantly slower than `filepath.WalkDir` due to the overhead of `os.Lstat` calls for every file and directory visited.
**Action:** Always prefer `filepath.WalkDir` when traversing directories in Go to improve performance, especially when there are many files.
