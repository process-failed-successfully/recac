1. **Implement `SkipJobDownstream` in `Orchestrator`**
   - Similar to `CancelJobDownstream` and `RetryJobDownstream`.
   - Take `jobID` and find all active/pending downstream dependencies via BFS.
   - For pending jobs, mark them as `Skipped`.
   - (For active jobs, skipping downstream would mean cancelling active ones and marking pending as skipped. If `skip` applies only to pending jobs currently, we might just skip the pending downstream jobs, or cancel the active ones and skip the rest. Looking at the logic, "skip" usually applies to pending jobs. Let's make it mark pending downstream jobs as skipped, and maybe skip active too if they haven't finished? Wait, `CancelJobDownstream` cancels active and pending. `SkipJobDownstream` should probably skip pending downstream jobs. Actually, let's look at `SkipJob`. It only skips *pending* jobs. So `SkipJobDownstream` should skip pending downstream dependencies. Wait, `CancelJobDownstream` cancels active AND pending. `RetryJobDownstream` retries completed jobs and their downstream. So `SkipJobDownstream` should skip the target pending job AND all its pending downstream jobs.)

2. **Add API Endpoint**
   - In `internal/orchestrator/api.go`, modify `POST /jobs/{id}/skip` to accept a `?downstream=true` query parameter.
   - If `downstream=true`, call `orch.SkipJobDownstream(ctx, id, logger)`, else call `orch.SkipJob(ctx, id, logger)`.

3. **Add CLI flag in `cmd/orchestrator`**
   - The user might want `--skip-downstream` or `cmd/orchestrator/main.go` `--downstream` used with `--skip-job`.
   - Update `main.go` and `submission.go` (`skipJob`) to support the `downstream` parameter.

4. **Update Web UI / TUI (if necessary/optional)**
   - The problem statement asks for one high-value feature. Adding downstream skip completes the symmetry with retry/cancel downstream.

5. **Write tests**
   - Add `TestSkipJobDownstream` in `orchestrator_skip_test.go` or similar.
