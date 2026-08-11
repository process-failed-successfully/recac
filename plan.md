1. **Optimize `jobsToCancel` slice pre-allocation in `internal/orchestrator/orchestrator.go`:**
   - At line 4028 in `processWorkItemInternal`, `jobsToCancel` is pre-allocated with `make([]string, 0, len(o.activeJobs))`.
   - Immediately after the loop over `o.activeJobs`, there is a loop over `o.pendingJobs` which also appends to `jobsToCancel`.
   - If there are items appended from `o.pendingJobs`, it causes dynamic reallocation overhead because the initial capacity only accounted for `o.activeJobs`.
   - I will change the pre-allocation to `jobsToCancel = make([]string, 0, len(o.activeJobs)+len(o.pendingJobs))` to ensure maximum capacity is reserved up-front, adhering to the "Pre-allocate Slice Capacity in Filter Iterations over Maps" learnings in Bolt's journal.

2. **Verify changes:**
   - Run tests `make test-local` and `go vet ./...`.

3. **Complete pre-commit steps:**
   - Complete pre-commit steps to ensure proper testing, verification, review, and reflection are done.

4. **Submit PR:**
   - Create PR with title "⚡ Bolt: Pre-allocate full slice capacity for jobsToCancel".
