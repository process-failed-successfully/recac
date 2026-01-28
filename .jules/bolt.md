## BOLT'S JOURNAL

## 2025-05-23 - [Repeated Map Allocation in Walkers]
**Learning:** `DefaultIgnoreMap` was allocating a new map for every directory visited in `filepath.Walk` callbacks across multiple commands (`breaking`, `pair`, `tree`). In deep file trees, this creates significant GC pressure.
**Action:** Use `sync.Once` to cache static lookup maps that are accessed in tight loops or recursive walkers.
