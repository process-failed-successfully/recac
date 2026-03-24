# RECAC Orchestrator

The Orchestrator is the management layer of the RECAC system. Its primary responsibility is to poll for work items (like Jira tickets or local files) and spawn Agent jobs to handle them.

## Key Features

- **Multi-Source Polling**: Supports Jira, GitHub, GitLab, and local filesystem.
- **Hybrid Spawning**: Can run agents locally via Docker or in Kubernetes via Jobs.
- **Auto-Retry**: Automatically cleans up and retries failed jobs.
- **Configurable**: Fully controllable via CLI flags and environment variables.

## Usage

```bash
./bin/orchestrator [flags]
```

### Core Flags

| Flag               | Env Var                       | Default      | Description                            |
| ------------------ | ----------------------------- | ------------ | -------------------------------------- |
| `--mode`           | `RECAC_ORCHESTRATOR_MODE`     | `local`      | `local` (Docker) or `k8s` (Kubernetes) |
| `--poller`         | `RECAC_POLLER`                | `jira`       | `jira`, `github`, `gitlab`, `file`     |
| `--interval`       | `RECAC_ORCHESTRATOR_INTERVAL` | `1m`         | Polling interval (e.g., `30s`, `5m`)   |
| `--agent-provider` | `RECAC_AGENT_PROVIDER`        | `openrouter` | AI provider for spawned agents         |
| `--agent-model`    | `RECAC_AGENT_MODEL`           | `...`        | AI model for spawned agents            |
| `--wait-job`       | `RECAC_ORCHESTRATOR_WAIT_JOB` | -            | Wait for a job to complete             |
| `--wait-tag`       | `RECAC_ORCHESTRATOR_WAIT_TAG` | -            | Wait for jobs by tag                   |
| `--wait-match`     | `RECAC_ORCHESTRATOR_WAIT_MATCH` | -            | Wait for jobs matching regex           |
| `--delete-pending-job` | - | - | Delete a specific job from the pending queue |
| `--delete-pending-tag` | - | - | Delete all pending jobs with the specified tag |
| `--delete-pending-match` | - | - | Delete all pending jobs matching the given regex |
| `--export-trace` | - | - | Export jobs as Chrome Trace Event format to a JSON file (use '-' for stdout) |
| `--export-trace-state` | - | `all` | State of jobs to export trace for (`all`, `active`, `completed`, `failed`) |

### Kubernetes Mode Flags

| Flag                  | Env Var                        | Default   | Description                        |
| --------------------- | ------------------------------ | --------- | ---------------------------------- |
| `--image`             | `RECAC_ORCHESTRATOR_IMAGE`     | `...`     | Docker image to use for Agent Jobs |
| `--namespace`         | `RECAC_ORCHESTRATOR_NAMESPACE` | `default` | K8s namespace for jobs             |
| `--image-pull-policy` | `RECAC_IMAGE_PULL_POLICY`      | `Always`  | `Always`, `IfNotPresent`, `Never`  |

### Jira Poller Flags

| Flag           | Env Var | Default       | Description                        |
| -------------- | ------- | ------------- | ---------------------------------- |
| `--jira-label` | -       | `recac-agent` | Poll for issues with this label    |
| `--jira-query` | -       | -             | Custom JQL query (overrides label) |
| `--jira-webhook-secret` | `RECAC_JIRA_WEBHOOK_SECRET` | - | Jira Webhook Secret for validating incoming POST events (passed as ?secret= parameter) |

### GitHub Poller Flags

| Flag | Env Var | Default | Description |
|---|---|---|---|
| `--github-token` | `RECAC_GITHUB_TOKEN` | - | GitHub API Token |
| `--github-owner` | `RECAC_GITHUB_OWNER` | - | GitHub Repository Owner |
| `--github-repo` | `RECAC_GITHUB_REPO` | - | GitHub Repository Name |
| `--github-label` | `RECAC_GITHUB_LABEL` | - | GitHub Label to poll for |

### Linear Poller Flags

| Flag | Env Var | Default | Description |
|---|---|---|---|
| `--linear-token` | `RECAC_LINEAR_TOKEN` | - | Linear API Token |
| `--linear-team` | `RECAC_LINEAR_TEAM` | - | Linear Team ID (e.g. `ENG`) |
| `--linear-label` | `RECAC_LINEAR_LABEL` | - | Linear Label to poll for (defaults to `jira-label`) |

### GitLab Poller Flags

| Flag | Env Var | Default | Description |
|---|---|---|---|
| `--gitlab-token` | `RECAC_GITLAB_TOKEN` | - | GitLab API Token |
| `--gitlab-project` | `RECAC_GITLAB_PROJECT` | - | GitLab Project ID or URL-encoded path |
| `--gitlab-label` | `RECAC_GITLAB_LABEL` | - | GitLab Label to poll for |
| `--gitlab-url` | `RECAC_GITLAB_URL` | `https://gitlab.com` | GitLab instance URL |

### Trello Poller Flags

| Flag | Env Var | Default | Description |
|---|---|---|---|
| `--trello-key` | `RECAC_TRELLO_KEY` | - | Trello API Key |
| `--trello-token` | `RECAC_TRELLO_TOKEN` | - | Trello API Token |
| `--trello-board` | `RECAC_TRELLO_BOARD` | - | Trello Board ID |
| `--trello-list` | `RECAC_TRELLO_LIST` | - | Trello List ID to poll for |
| `--trello-webhook-secret` | `RECAC_TRELLO_WEBHOOK_SECRET` | - | Trello Webhook Secret for validating incoming POST events |

### Generic Webhook Flags

| Flag | Env Var | Default | Description |
|---|---|---|---|
| `--generic-webhook-secret` | `RECAC_GENERIC_WEBHOOK_SECRET` | - | Secret for validating incoming POST events to `/webhook/generic` via the `X-Webhook-Signature` header (HMAC-SHA256) |

### File Poller Flags

| Flag          | Env Var           | Default           | Description                      |
| ------------- | ----------------- | ----------------- | -------------------------------- |
| `--work-file` | `RECAC_WORK_FILE` | `work_items.json` | Path to the JSON work items file |

## Operational Modes

### Local Mode (`--mode local`)

In local mode, the orchestrator connects to your local Docker daemon and spawns agents as individual containers. This is ideal for development and small-scale automation on a single machine.

### Kubernetes Mode (`--mode k8s`)

In K8s mode, the orchestrator creates `batch/v1` Jobs within the cluster. This is designed for production environments where you need high availability and horizontal scaling.

## Work Delivery

### Jira Poller

The orchestrator searches for issues matching the label and ensures they aren't already completed (`statusCategory != Done`). It passes the ticket description and metadata directly to the spawned agent.

### File Poller

Expects a JSON file with the following structure:

```json
[
  {
    "id": "TASK-1",
    "summary": "Implement login",
    "description": "...",
    "repo_url": "..."
  }
]
```

### Generic Webhook

You can submit custom jobs directly to the orchestrator via a generic webhook endpoint. Send a `POST` request to `/webhook/generic` with a JSON payload matching the `WorkItem` structure.

```bash
curl -X POST http://localhost:2112/webhook/generic \
  -H "Content-Type: application/json" \
  -d '{
    "id": "custom-job-1",
    "summary": "Custom task",
    "description": "Run custom scripts",
    "repo_url": "https://github.com/org/repo"
  }'
```

If `--generic-webhook-secret` is set, you must include an `X-Webhook-Signature` header with the HMAC-SHA256 signature of the payload.
