If `--keep-workspace` or `--retain-workspace` (bool in WorkItem `RetainWorkspace bool`) is true, the `DockerSpawner` (and `ProcessSpawner`) will NOT remove `tempDir` after the agent finishes. This is hugely valuable for debugging what the agent did locally before failing (since it might have created files but not pushed them).
Wait, the `Janitor` will have to clean them up eventually, or the user can.
But maybe this is too simple?

What about **Dynamic Scaling based on queue size**?
The orchestrator currently has `orch.SetConcurrency(ctx, max)`.
If we could auto-scale the maximum concurrent jobs based on pending queue size, that would be cool. But concurrency is often limited by resources/Docker CPU, not queue size.

What about **Rate Limiting or Throttling job submissions**?
Maybe.

What about **Bulk "Rerun" (duplicate) jobs from History**?
Wait, `POST /jobs/clone/bulk` exists and CLI is `--clone-match` / `--clone-tag`. It essentially re-runs them with new IDs.

What about **Viewing the Orchestrator's internal Queue Order**?
Like `GET /jobs/queue` which returns pending jobs ordered exactly by priority and delay. `list-pending` already does this, but maybe not explicitly sorted?
Let's check if `orch.GetPendingJobs()` sorts by priority.
