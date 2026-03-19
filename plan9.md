Yes, the `Spawner` interface lacks `Pause` and `Resume`.
I can add `PauseJob(ctx context.Context, jobID string) error` and `ResumeJob(ctx context.Context, jobID string) error` to the `Spawner`.

In `spawner_docker.go`, `DockerClient` lacks `PauseContainer` / `UnpauseContainer`. I'll add them to `DockerClient` interface and implementation.
In `spawner_process.go`, since we use `exec.Cmd`, we can send `SIGSTOP` and `SIGCONT`.
In `spawner_k8s.go`, Kubernetes doesn't natively support pausing a Job easily (it can be scaled down or suspended, though suspending a running Job doesn't stop the pod, it terminates it if you change `.spec.suspend` usually, but maybe we just return `ErrUnsupported` for K8s).
Wait, Kubernetes `Job` has a `.spec.suspend` field which can pause a job, though it usually scales the active pods to 0 (terminates them). If so, we just return `ErrUnsupported` for `PauseJob` in K8s, or implement it if it's easy.

This is a true "bold and innovative" feature: **Granular Job Execution Control (Pause/Resume individual jobs dynamically).**

Let's check if there is an alternative idea that's better:
- **Global `tags` aggregation**: `GET /tags` across all jobs, with usage count.
  Why? To power analytics, filtering drop-downs in the UI, and CLI autocomplete. Very useful.
- **Support for importing bulk jobs**: `POST /jobs/import` to upload a JSON array of complete jobs (with their history/state) from `export/jobs` to seamlessly migrate orchestrator instances or backup/restore. The orchestrator CLI has `--import-jobs`, let's see how it works.
