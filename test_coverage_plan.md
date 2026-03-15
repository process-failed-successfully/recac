1. **Analyze Coverage Gaps:**
   - I checked overall coverage using `go tool cover -func=coverage.out` after running tests for multiple packages (`cmd/recac`, `cmd/orchestrator`, `internal/tui`, `internal/orchestrator`).
   - `recac/cmd/recac` currently has an extremely low coverage of 5.6%.
   - `recac/cmd/orchestrator` has around 74.6% coverage, with `renameJob` at 0% coverage.
   - `recac/internal/tui` has 79.3% coverage.

2. **Select Target Areas:**
   - To immediately improve overall coverage meaningfully, I will focus on writing tests for `cmd/orchestrator/submission.go` (specifically `renameJob` and `skipJob` which are entirely or partially missing tests).
   - `cmd/recac/start.go` -> `processDirectTask` can also be improved. I will add tests for `processDirectTask`.

3. **Plan for implementation:**
   - Add `TestRenameJob` in `cmd/orchestrator/submission_test.go` checking successful rename and failure modes.
   - Add `TestProcessDirectTask` in `cmd/recac/start_test.go` checking execution flow when creating temp space and running workflow.
   - Re-run `make cover` or `go test ...` locally to verify changes, increasing overall coverage to exceed 80% (right now it's around 82.5% according to the latest run over all packages, but we need to ensure the overall project stays comfortably above 80%).

4. **Refine:**
   - I noticed the overall `go test ./...` returns around 82.5% coverage. Wait, `recac/cmd/recac` had 5.6%. I need to increase coverage to make the *entire project* strictly robust, so I will add tests for the specific functions with 0% coverage.
