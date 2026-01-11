# recac - Rewrite of Combined Autonomous Coding

> [!WARNING] > **BINARY DEPRECATION**: The single `recac` binary is now deprecated.
> Please use the new distributed binaries:
>
> - **`orchestrator`**: For polling tasks (Jira/Files) and spawning agents.
> - **`recac-agent`**: The core autonomous coding agent.

`recac` is a comprehensive framework for autonomous coding. It has evolved from a single CLI tool into a distributed architecture designed for scale and reliability, particularly in Kubernetes environments.

## Features

- **🚀 Distributed Architecture**: Separate Orchestrator and Agent binaries for independent scaling.
- **🤖 Multi-Agent System**: Built-in support for Gemini, OpenAI, and Ollama.
- **🐳 Docker & K8s Native**: Run agents locally in Docker or as distributed Jobs in Kubernetes.
- **📋 Feature Tracking**: End-to-End feature lifecycle (Spec -> Implementation -> Verification).
- **📊 Real-time Monitoring**: TUI and Slack/Discord integrations for progress tracking.
- **🔄 Smart Orchestration**: Automatic Jira polling and agent lifecycle management.

## Project Structure & Documentation

The project is now divided into specialized components. Please refer to the specific documentation for each:

- [**Orchestrator**](file:///home/luke/repos/recac/cmd/orchestrator/README.md): How to manage the task pool and agent spawning.
- [**RECAC Agent**](file:///home/luke/repos/recac/cmd/agent/README.md): How the autonomous coding logic works and how to run it manually.
- [**E2E Testing**](file:///home/luke/repos/recac/e2e/README.md): Documentation for the scenario-based testing framework.
- [**Helm/K8s**](file:///home/luke/repos/recac/deploy/helm/recac/README.md): Guide for deploying to Kubernetes.

## Quick Start

### Prerequisites

- **Go 1.21+**
- **Docker** and **Docker Compose**
- **Git**
- **Kubernetes (Optional)** (e.g., k3s, minikube)

### Installation

```bash
# Clone the repository
git clone https://github.com/process-failed-successfully/recac.git
cd recac

# Build everything
make build
```

The `make build` command will produce:

- `bin/orchestrator`
- `bin/recac-agent`
- `bin/recac` (Legacy/CLI wrapper)

### Configuration

Create a `.recac.yaml` in your home directory.

```yaml
# AI Provider
agent_provider: openrouter
agent_model: "mistralai/devstral-2512:free"
api_key: "your-api-key"

# Integrations
jira_url: "https://your-domain.atlassian.net"
jira_email: "user@example.com"
jira_token: "api-token"
```

## Usage (Distributed Mode)

### 1. Run the Orchestrator

The orchestrator polls for work and starts agents.

```bash
# Local Docker mode
./bin/orchestrator --mode local --jira-label "recac-agent"
```

### 2. The Agent

The agent is usually spawned by the orchestrator, but can be run manually for debugging:

```bash
./bin/recac-agent --jira RD-123 --project "My Project" --repo-url "https://github.com/org/repo"
```

## Architecture

`recac` utilizes a **Poll-Spawn-Verify** loop:

1.  **Orchestrator**: Watches Jira (or a file) for new tasks matching a specific label.
2.  **Spawner**: Creates a Docker container or K8s Job for each task.
3.  **Agent**: Clones the repo, analyzes the task, implements the code, and pushes back.
4.  **Verification**: The agent runs QA checks and the manager signs off before completion.

```mermaid
graph TD
    J[Jira/Backlog] -->|Poll| O[Orchestrator]
    O -->|Spawn| A1[Agent Pod 1]
    O -->|Spawn| A2[Agent Pod 2]
    A1 -->|Git Push| G[GitHub/GitLab]
    A2 -->|Git Push| G
    A1 -->|Notify| S[Slack/Discord]
```

## End-to-End Example: Hello World

1.  **Init**: Create a new folder and run `recac init`.
2.  **Spec**: Edit `app_spec.txt` and add:
    ```text
    Task: Create a file named hello.go that prints "Hello, Autonomous World!".
    ```
3.  **Run**: Execute `recac start`.
    - The agent analyzes the request.
    - It spins up a Docker container.
    - It writes the Go code inside the container.
    - It runs `go run hello.go` to verify output.
4.  **Verify**: Check your workspace for `hello.go`.

### Deployment (Experimental)

To deploy the orchestrator in a Kubernetes cluster using Helm:

```bash
helm install recac ./deploy/helm/recac \
  --set secrets.apiKey=$API_KEY \
  --set config.jiraUrl=$JIRA_URL \
  --set config.jiraUsername=$JIRA_EMAIL \
  --set secrets.jiraApiToken=$JIRA_API_TOKEN
```

> [!NOTE]
> The orchestrator requires access to the Docker daemon to run agents. By default, the Helm chart mounts `/var/run/docker.sock` from the host. Ensure your Kubernetes nodes have Docker installed and the socket is accessible.

## Architecture

`recac` is built with a modular architecture:

- **cmd/**: CLI entry points (Cobra).
- **internal/agent/**: AI provider abstractions.
- **internal/runner/**: The core workflow loop and state management.
- **internal/docker/**: Docker API client wrappers.
- **internal/ui/**: TUI components (Bubble Tea).

## License

MIT
