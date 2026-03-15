1. **Understand Goal**: Increase test coverage in `recac` beyond 80%.
2. **Current state**: Overall test coverage is roughly 82.1%, but `cmd/recac` currently has a very low coverage of ~5.7%.
3. **Execution Plan**:
    - **Step 1**: Implement tests for `cmd/recac/focus.go` (currently 18.2%). Since `runtime.GOOS` is a constant and difficult to mock, we'll abstract it out if needed or just accept the platform-specific coverage limitation, but we will test all other logic like `runFocus` without a task, key bindings in `Update()`, etc. Verify coverage using `go test ./cmd/recac -coverprofile=coverage.out -run TestFocus && go tool cover -func=coverage.out | grep focus.go`.
    - **Step 2**: Explore and test `cmd/recac/workdiff.go` (currently 22.2%). Use `read_file` to understand its logic, then create specific tests validating logic for comparing git branch outputs by executing commands in mock environments using `test_helpers` or mock structs present. Verify coverage.
    - **Step 3**: Explore and test `cmd/recac/gym.go` (currently 25.0%). Investigate using `read_file`. Then, provide granular tests outlining the specific behaviors and mock inputs to be tested. Verify coverage.
    - **Step 4**: Run the full test suite (`make test` or `go test ./...`) and lint checks (`make lint`) to ensure existing tests pass and no regressions were introduced.
    - **Step 5**: Complete pre commit steps to ensure proper testing, verification, review, and reflection are done.
    - **Step 6**: Submit the changes.
