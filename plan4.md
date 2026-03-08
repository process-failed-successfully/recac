Ah, `Follow: true` is set. So it IS a live stream.
Wait, let's look at another idea. What if we add a subcommand to get the history/status of the orchestrator as a nice TUI view instead of the full dashboard? The full dashboard is bubble tea.
What if we add a feature to **gracefully drain** jobs from the orchestrator? A "Drain" mode where the orchestrator stops polling new work but finishes the existing jobs before exiting? That's very useful for maintenance!
Right now, `cancel-all` cancels everything. But `pause` pauses polling, and then what? If you pause, it finishes active jobs. But what if you want to cleanly exit once they're done?
Actually, wait. The `recac` CLI itself has been deprecated in favor of `orchestrator` and `recac-agent`.

Wait, the orchestrator handles `Jobs`. What if we add a feature to **Re-evaluate pending jobs** via an endpoint/CLI flag? It does it automatically, but maybe we want to force it?
Or what about **Updating environment variables** for a pending job? Right now we can clone it with overrides.

Let's look at `internal/orchestrator/api.go` for what APIs exist.
- `GET /healthz`
- `GET /status`
- `GET /jobs` (active or all)
- `GET /jobs/export`
- `GET /jobs/{id}`
- `GET /jobs/{id}/logs`
- `PUT /jobs/{id}/priority`
- `POST /jobs/{id}/retry`
- `POST /jobs/{id}/clone`
- `POST /jobs/retry-failed`
- `DELETE /jobs/{id}`
- `DELETE /jobs`
- `DELETE /pending`
- `DELETE /history/{id}`
- `DELETE /history`
- `POST /poll`
- `POST /jobs`
- `POST /jobs/batch`
- `POST /webhook/github`
- `POST /webhook/gitlab`
- `POST /pause`
- `POST /resume`
- `POST /scale`

What's missing?
1. **Webhook support for Bitbucket** or other SCMs?
2. **Search / Filter Jobs endpoint**: `GET /jobs?state=completed&status=Failed&limit=10`. This is super useful because right now `GET /jobs?state=all` returns EVERYTHING, which could be huge if history is large (max 50 by default, though).
Wait, filtering is nice but maybe too simple.
3. **Job Execution Time Limits (Timeout) adjustment**: `--update-timeout`?
4. **Agent Provider/Model override per Job**: Currently, `WorkItem` has `EnvVars` which can override `RECAC_AGENT_PROVIDER` if the Spawner respects it.
Wait! Does the DockerSpawner read `EnvVars["RECAC_AGENT_PROVIDER"]`? Let's check.
