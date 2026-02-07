## BOLT'S JOURNAL

## 2026-02-07 - High Latency in DB Lock Acquisition
**Learning:** `AcquireLock` in both Postgres and SQLite implementations used a 500ms polling interval. In a multi-agent system where agents frequently contend for locks (e.g., file locks), this adds significant latency (avg 250ms per contention event). Reducing this to 50ms makes the system feel much snappier without significant DB load.
**Action:** When implementing polling loops for locks or status checks, default to a lower interval (e.g., 50-100ms) for interactive/high-concurrency paths, or use notification mechanisms (like Postgres LISTEN/NOTIFY) if possible.

## 2026-02-07 - Git Commit Failures
**Learning:** `git commit` exits with code 1 if there are no changes to commit. In an automated agent loop, this is often interpreted as a failure, causing unnecessary retries and eventual rate limiting.
**Action:** When executing `git commit` in an automated context, check the output for "nothing to commit" or "working tree clean" and treat it as a success (idempotent operation) rather than a hard failure.
