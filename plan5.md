Let's see if we can add a new feature: **`export-graph-state` for exporting graph in different states (like only failed/completed/pending)**. Oh wait, `GET /jobs/export/graph` already takes `state` query param! The CLI `--export-graph` does not expose state.

What if we add a **"Replay" feature for Orchestrator Events**?
Currently `/events` streams SSE real-time. But there is no way to get past events.

How about **Adding Environment Variable filtering (`--env-filter`) to `list-jobs` / `list-pending` and `/jobs` API**?
If someone submits 50 matrix jobs that just vary by `TEST_SUITE=unit`, `TEST_SUITE=integration`, etc., and they want to see the status of the "integration" jobs, they have NO WAY to filter them via CLI or API.
You can filter by `match` (which searches summary/error) or `tag`, but not `env`.

Let's verify `GET /jobs` again.
It accepts `state`, `status`, `tag`, `match`.
If we add `env` parameter, e.g., `?env=TEST_SUITE=integration`
This is highly useful for Matrix jobs!
Is it bold and innovative enough? Maybe.

Another very useful idea: **Job "Suspend" and "Resume" at runtime** for the Docker Spawner/Process Spawner.
Docker supports `docker pause` and `docker unpause`. Process supports `SIGSTOP` / `SIGCONT`.
Currently, the Orchestrator has `/pause` and `/resume` which pauses the *Polling Loop*. It does NOT pause the currently executing jobs!
If a user realizes their DB is down, they might want to pause all *running* agents, or a specific agent, and then resume it.
This would be a killer feature!
Let's check if `DockerClient` supports `PauseContainer` / `UnpauseContainer`.
