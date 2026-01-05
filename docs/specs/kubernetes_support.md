# Specification: Kubernetes Operator Support for Agents

## 1. Overview

This specification outlines the evolution of `recac` into a Kubernetes-native Orchestrator (Operator). Instead of just abstracting Docker locally, `recac` will run as a long-lived service within a Kubernetes cluster. It will continuously discover work (starting with Jira tickets) and spawn ephemeral Kubernetes Jobs to perform tasks.

Recac repository: https://github.com/process-failed-successfully/recac

There is existing work in `deploy/helm/recac` ensure you build upon this.

## 2. Architecture

### 2.1 The Orchestrator (Operator)

The `recac` core service runs as a Kubernetes **Deployment**.

- **Role**: Controller / Operator.
- **Responsibility**:
  1.  Poll/Listen for work triggers (Jira, Webhooks, Cron).
  2.  Manage the lifecycle of Agent Jobs.
  3.  Aggregate logs and status.

### 2.2 The Agent (Worker)

Agents are spawned as Kubernetes **Jobs**.

- **Isolation**: Each task runs in its own isolated Pod/Job.
- **Ephemeral**: The environment is created for the task and destroyed afterwards.
- **Self-Contained**: The Agent container is responsible for setting up its own workspace (git clone).

## 3. Work Discovery

### 3.1 Source: Jira (Phase 1)

The Orchestrator will focus on Jira integration as the primary work source.

1.  **Poll**: Periodically query Jira (JQL) for tickets in a "Ready" or specific state (e.g., `labels = recac-agent`).
2.  **Claim**: Transition the ticket to "In Progress" or assign it to the bot user to prevent duplicate processing.
3.  **Dispatch**: Spawn a Kubernetes Job to handle the ticket.

## 4. Execution Flow

### 4.1 Job Lifecycle

1.  **Spawn**: The Orchestrator creates a `batch/v1 Job`.

    - **Env Vars**: Passes Ticket ID, Repo URL, and temporary credentials.
    - **Secrets**: Mounts necessary secrets (Github Token, Jira Token, OpenAI/Anthropic Keys).

2.  **Initialization (The "Clone" Step)**:

    - Unlike the previous design (Shared PVC), each Agent **clones the repository** it needs at startup.
    - _Advantages_: No stale state, clean slate for every run, supports multiple different repos easily.
    - _Mechanism_: An `initContainer` or the first step of the Agent entrypoint performs `git clone <repo_url>`.

3.  **Execution**:

    - The Agent starts usually with `recac start --jira <TICKET_ID>`.
    - It creates a feature branch, performs work, runs tests, and pushes changes.

4.  **Completion**:
    - **Success**: Agent transitions Jira ticket to "Review/Done". Job completes with Exit Code 0.
    - **Failure**: Agent comments on Jira with error logs. Job fails. Orchestrator may retry or alert.

### 4.2 Workspace Persistence

- **Ephemeral**: The workspace exists only for the duration of the Job.
- **Persistence**: Any changes must be pushed to Git to be saved.
- **Logs/Artifacts**: Logs are streamed to the Orchestrator (or centralized logging like Loki) and potentially attached to the Jira ticket.

## 5. Kubernetes Resources

### 5.1 Deployment (Orchestrator)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: recac-orchestrator
spec:
  replicas: 1 # Can be scaled >1 with Leader Election (see HA)
  template:
    spec:
      serviceAccountName: recac-operator-sa
      containers:
        - name: orchestrator
          image: ghcr.io/org/recac:latest
          command: ["recac", "orchestrate"]
          env:
            - name: JIRA_URL
              value: "https://..."
```

### 5.2 Job (Agent Template)

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  generateName: recac-agent-
spec:
  ttlSecondsAfterFinished: 3600 # Auto-cleanup
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: agent
          image: ghcr.io/org/recac-agent:latest
          command: ["/bin/sh", "-c"]
          args:
            - |
              git clone https://$GITHUB_TOKEN@github.com/org/repo.git .
              recac start --jira $JIRA_TICKET
```

## 6. High Availability & Resilience

### 6.1 Orchestrator HA

To ensure the Orchestrator itself is not a single point of failure:

- **Replicas**: The Deployment should be configured with `replicas: 2` (or more).
- **Leader Election**: Use `client-go`'s `leaderelection` package (via `Lease` API) to ensure only one active leader is polling Jira and spawning jobs at a time.
- **Failover**: If the leader crashes, a standby immediately acquires the lease.

### 6.2 Job Resilience

- **Idempotency**: Agents must designed to be idempotent. If a Job is restarted (e.g. node failure), the agent should detect if:
  - Local work was started (it won't exist in a new pod).
  - Branch already exists remotely (fetch and checkout instead of create).
  - Jira ticket is already "In Progress" (verify own identity/claim).
- **Retries**:
  - Use `backoffLimit` in Job spec for transient failures (e.g., network issues during git clone).
  - Orchestrator monitors for permanent failures (Exit Code > 0 after retries) and updates Jira accordingly.

### 6.3 Recovery & State

- **Orphan Adoption**: On startup (or new leader election), the Orchestrator must scan for existing active Jobs labeled `app=recac-agent`.
- **Status Sync**: It should verify the status of these Jobs against the known state in Jira. If a Job is dead but the ticket is "In Progress", it should either respawn or mark as failed.

## 7. Observability

### 7.1 Prometheus Metrics

The Orchestrator will expose a `/metrics` endpoint for Prometheus scraping.

- **Standard Metrics**:

  - `go_*`: Standard Go runtime metrics (goroutines, GC, memory).
  - `process_*`: Standard process metrics (CPU, open FDs).

- **Custom Business Metrics**:
  - `recac_jobs_active`: Gauge, current number of running agent jobs.
  - `recac_jobs_created_total`: Counter, total jobs spawned.
  - `recac_jobs_completed_total`: Counter, jobs that finished with exit code 0.
  - `recac_jobs_failed_total`: Counter, jobs that failed or timed out.
  - `recac_jira_api_requests_total`: Counter, labeled by `method` and `status_code`.

### 7.2 Structured Logging

All components (Orchestrator and Agents) must use structured logging (JSON format) to facilitate ingestion by log aggregators (e.g., Fluentd, Loki).

- **Library**: `log/slog` (Go standard library).
- **Format**: JSON.
- **Required Fields**:
  - `level`: (INFO, WARN, ERROR, DEBUG)
  - `ts`: Timestamp in ISO 8601.
  - `component`: `orchestrator` | `agent`
  - `job_id`: (If applicable) Kubernetes Job name.
  - `ticket_id`: (If applicable) Jira Ticket Key (e.g., PROJ-123).
  - `correlation_id`: Unique ID to trace execution flows.

Example:

```json
{
  "level": "INFO",
  "ts": "2023-10-27T10:00:00Z",
  "component": "orchestrator",
  "msg": "Spawning new agent job",
  "ticket_id": "PROJ-123",
  "job_name": "recac-agent-abcde"
}
```

## 8. Migration & Roadmap

1.  **Phase 1: Jira Poller**: Implement the polling logic in `cmd/recac` (new `orchestrate` command).
2.  **Phase 2: Job Spawner**: Implement the K8s client logic to create Jobs dynamically.
3.  **Phase 3: HA & Election**: Implement leader election for the orchestrator.
4.  **Phase 4: Feedback Loop**: Ensure Job logs/status make it back to Jira.
