# RECAC Orchestrator

The Orchestrator is the management layer of the RECAC system. Its primary responsibility is to poll for work items (like Jira tickets or local files) and spawn Agent jobs to handle them.

## Key Features

- **Multi-Source Polling**: Supports Jira and local filesystem.
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
| `--poller`         | `RECAC_POLLER`                | `jira`       | `jira` or `file`                       |
| `--interval`       | `RECAC_ORCHESTRATOR_INTERVAL` | `1m`         | Polling interval (e.g., `30s`, `5m`)   |
| `--agent-provider` | `RECAC_AGENT_PROVIDER`        | `openrouter` | AI provider for spawned agents         |
| `--agent-model`    | `RECAC_AGENT_MODEL`           | `...`        | AI model for spawned agents            |

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
