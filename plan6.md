How about a new feature: **Job Artifacts Extraction**.
Wait, the `getLogs` API pulls logs. But `export/archive` bundles `job.json` and `logs.txt` into a `.tar.gz`. What if the agent produced output files that need to be retrieved from the workspace? The workspace gets deleted!
"Clean up workspace: `os.RemoveAll(tempDir)`"
If a job completes, its artifacts are gone. Wait, the README mentions `recac-agent` pushes to git.
But what if the user wants to keep the workspace for a failed job to debug?
Orchestrator currently ALWAYS runs `os.RemoveAll(tempDir)`.
Wait, adding `--keep-workspace` or `KeepWorkspace: true` to `WorkItem` would be amazing for debugging!
Let's check if there is a `KeepWorkspace` or something.
