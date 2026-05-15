1. **Analyze:** The memory specifies: *The orchestrator API and CLI support updating job properties such as priority, tags, and max retries via PUT endpoints (e.g., `/jobs/{id}/priority`, `/jobs/{id}/tags`, `/jobs/{id}/max-retries`), along with bulk update capabilities for priority and tags using query parameters (e.g., `PUT /jobs/priority?tag=...`).*
However, while `promote` and `demote` actions support bulk operations by `group` (e.g., `--promote-group`, `PUT /jobs/promote/bulk?group=...`), the bulk `update-priority` CLI tool and the `/jobs/priority` endpoint **only** support `tag` and `match`.

2. **Feature:** Add `--update-priority-group` to the CLI, and support `group` query parameter in `PUT /jobs/priority` endpoint to allow bulk updating priority by concurrency group.

3. **Implementation Plan:**
   - **`cmd/orchestrator/main.go`:** Add `--update-priority-group` flag and correctly map it. Use `updateBulkPriority` with `group` parameter.
   - **`cmd/orchestrator/update_priority.go`:** Modify `updateBulkPriority(host, match, tag string, priority int)` to `updateBulkPriority(host, match, tag, group string, priority int)` and pass `group` to the `PUT /jobs/priority` API.
   - **`internal/orchestrator/api.go`:** Modify `PUT /jobs/priority` to accept `group` query param. Use `UpdateJobsPriorityByGroup` in `Orchestrator`.
   - **`internal/orchestrator/orchestrator.go`:** Implement `UpdateJobsPriorityByGroup(ctx context.Context, group string, newPriority int, logger *slog.Logger) (int, error)`.
   - **`internal/orchestrator/orchestrator_priority_extra_test.go`:** Add test for `UpdateJobsPriorityByGroup`.
   - **`internal/orchestrator/api_test.go` or `api_priority_test.go`:** Add test for the `PUT /jobs/priority` with `group`.
   - **`cmd/orchestrator/main_actions_test.go` (or similar):** Verify new flag.

4. **Verify tests:** Run unit tests for `orchestrator` and `cmd/orchestrator` and e2e testing.

5. **Pre-commit and Submit.**
