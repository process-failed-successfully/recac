# RECAC Helm Chart

This Helm chart deploys the RECAC (Rewrite of Combined Autonomous Coding) orchestrator on a Kubernetes cluster.

## Introduction

RECAC is an autonomous coding assistant that manages development environments via Docker. This chart facilitates its deployment in a Kubernetes environment, enabling it to schedule coding tasks and manage agent lifecycles.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- Docker daemon accessible on Kubernetes nodes (for default agent execution)

## Installing the Chart

To install the chart with the release name `recac`:

```bash
helm install recac ./deploy/helm/recac \
  --set secrets.apiKey=$API_KEY \
  --set secrets.jiraApiToken=$JIRA_API_TOKEN
```

## Uninstalling the Chart

To uninstall/delete the `recac` deployment:

```bash
helm uninstall recac
```

## Configuration

The following table lists the configurable parameters of the RECAC chart and their default values.

| Parameter                  | Description                                 | Default                               |
| -------------------------- | ------------------------------------------- | ------------------------------------- |
| `replicaCount`             | Number of replicas for the orchestrator     | `1`                                   |
| `image.repository`         | Orchestrator image repository               | `recac`                               |
| `image.tag`                | Orchestrator image tag                      | `""` (defaults to `Chart.appVersion`) |
| `image.pullPolicy`         | Image pull policy                           | `IfNotPresent`                        |
| `serviceAccount.create`    | Create a ServiceAccount                     | `true`                                |
| `rbac.create`              | Create RBAC roles (required for K8s agents) | `true`                                |
| `dockerSocket.enabled`     | Mount host Docker socket                    | `true`                                |
| `dockerSocket.hostPath`    | Path to host Docker socket                  | `/var/run/docker.sock`                |
| `config.provider`          | AI Agent provider                           | `gemini`                              |
| `config.metricsPort`       | Port for metrics                            | `9090`                                |
| `config.maxIterations`     | Max agent iterations                        | `20`                                  |
| `config.managerFrequency`  | Frequency of manager reviews                | `5`                                   |
| `config.maxTokens`         | Max tokens per request                      | `32000`                               |
| `config.jiraUrl`           | Jira instance URL                           | `""`                                  |
| `config.jiraUsername`      | Jira username/email                         | `""`                                  |
| `secrets.apiKey`           | Generic API key                             | `""`                                  |
| `secrets.geminiApiKey`     | Gemini specific API key                     | `""`                                  |
| `secrets.openaiApiKey`     | OpenAI specific API key                     | `""`                                  |
| `secrets.openrouterApiKey` | OpenRouter specific API key                 | `""`                                  |
| `secrets.jiraApiToken`     | Jira API Token                              | `""`                                  |

## Security and Secrets

Sensitive parameters like API keys and tokens are stored in a Kubernetes Secret. You should provide these values during installation using `--set` or a custom `values.yaml` file.

Example using a private `values.yaml`:

```yaml
secrets:
  geminiApiKey: "your-api-key"
  jiraApiToken: "your-jira-token"
```

## Docker Integration

By default, the orchestrator mounts the host's Docker socket (`/var/run/docker.sock`) to allow it to run agent containers on the same node. This requires the Kubernetes nodes to have Docker installed and the orchestrator pod to have sufficient permissions.

## RBAC Permissions

The chart provisions a `ServiceAccount` and a `Role` with permissions to manage `batch/jobs` and `pods`. This is intended for future features where agents run as native Kubernetes Jobs instead of standalone Docker containers.
