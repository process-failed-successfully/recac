Wait, if we add `env_key` / `env_value` to `GET /jobs`, we should also add it to `listJobs` in `cmd/orchestrator/main.go` as `--list-jobs-env "KEY=VALUE"`.

Is there something even more innovative?
How about **Job Dependencies Deadlock Detection**? If Job A depends on Job B, and Job B depends on Job A, the orchestrator should detect this cycle and fail them. Wait, maybe there's a cycle detector already. Let's check `internal/orchestrator/orchestrator_dependency_test.go` or `orchestrator_cycle_test.go`.

Let's check `internal/orchestrator/orchestrator_cycle_test.go`:
Yes, cycle detection exists! `TestCircularDependency` in `orchestrator_cycle_test.go` probably checks it.

How about **Job Execution Timeout Configuration per Job**?
Wait, there is `--job-timeout` global flag, and `UpdateJobTimeout` API, and `--update-timeout-job`.

How about **Job Resource Limits** (CPU/Memory) for Docker/K8s spawner?
Let's check if `WorkItem` supports CPU/Memory limits.
