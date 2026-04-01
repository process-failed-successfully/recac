1. **Increase Test Coverage in `recac/internal/orchestrator`**
   - We observed that `CancelJobsByConcurrencyGroup` in `orchestrator.go` has 0% coverage.
   - I have written unit tests `TestOrchestrator_CancelJobsByConcurrencyGroup` and `TestOrchestrator_CancelJobsByConcurrencyGroup_Error` that increase coverage to 100% for this function.
   - I have also added `TestOrchestrator_SubmitJobWithConcurrencyGroupCancel`, `TestOrchestrator_ProcessWorkItemInternal_ConcurrencyGroupActive`, `TestOrchestrator_ProcessWorkItemInternal_Draining`, `TestOrchestrator_ProcessWorkItemInternal_AlreadyPendingApproval`, `TestOrchestrator_ProcessWorkItemInternal_AlreadyPendingDependencies`, `TestOrchestrator_ProcessWorkItemInternal_ConcurrencyGroupCancelActive`, `TestOrchestrator_ProcessWorkItemInternal_RetryCount`, `TestOrchestrator_ProcessWorkItemInternal_RunAfterCancel`, `TestOrchestrator_ProcessWorkItemInternal_RunAfterCancelError`, and `TestOrchestrator_ProcessWorkItemInternal_MaxRetriesFallback` tests for `processWorkItemInternal`.
   - The test issues stem from the fact that `o.SubmitJob` calls `o.processWorkItemInternal` which in turn fires a goroutine that calls `mockSpawner.Spawn()`. If we expect it to not panic, we must mock `mockSpawner.Spawn`.
2. **Increase Coverage for `api.go`**
   - I have already increased `GetNewAgentFunc` and `SetNewAgentFunc` to 100% coverage via `api_agent_test.go`.
3. **Verify and Adjust Tests**
   - Adjust `orchestrator_concurrency_test.go` to cleanly pass and mock `Spawn()` properly to prevent panics and test failures.
   - Run `make test` and `make cover` to ensure overall coverage meets >80% constraint.
4. **Complete pre-commit steps to ensure proper testing, verification, review, and reflection are done.**
5. **Submit**
