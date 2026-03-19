Ah, I see `MaxRetries`, `RunCondition`, etc.

What if we add **Job Webhooks**? A feature that allows a specific job to declare a `WebhookURL` in its `WorkItem`, and when the job status changes (e.g., completes, fails), the orchestrator POSTs the job JSON to that URL.
There is a generic webhook: `notifications.webhook.url` in config, which posts global events.
But having **per-job webhooks** is a common pattern in CI/CD platforms (e.g. GitHub Actions callback, or custom reporting).

Let's check if there is a `WebhookURL` in `WorkItem`. No.

Wait, another idea: **Step-by-step pipeline execution visualization / dry-run execution graph**. We already have `/export/graph`.

Let's think about **Bulk Job Import/Export via API**. Wait, `GET /jobs/export` exists. What about importing?
CLI has `--import-jobs`.

Let's look at `internal/orchestrator/pipeline.go`.
Does the orchestrator support **cron-like scheduled jobs** built-in as a poller?
Yes, `poller_cron.go`.

What about **Resource limits (CPU/Memory)** for jobs? Docker and K8s both support it easily.
If we add `CPULimit` (string, e.g., "500m" or "1") and `MemoryLimit` (string, e.g., "512Mi" or "1G") to `WorkItem`, the `DockerSpawner` and `K8sSpawner` can enforce these limits.
This is a standard CI/CD feature and missing here!
Let's see if we can do this.
Docker limits:
`container.HostConfig.Resources.Memory` and `NanoCPUs`
K8s limits:
`corev1.ResourceRequirements`

Let's look at `internal/orchestrator/spawner_docker.go` and `internal/orchestrator/spawner_k8s.go`.
