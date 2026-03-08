A **high-value feature**: Add a `--follow` flag or a `/jobs/{id}/stream` endpoint for logs that actually **streams logs live** instead of just reading what's currently there and closing?
Wait, `GetLogs` on Docker/K8s:
Let's see `internal/orchestrator/spawner_docker.go` and `spawner_k8s.go`.
If `GetLogs` streams the logs or just returns a snapshot.
