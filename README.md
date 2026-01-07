# recac - Rewrite of Combined Autonomous Coding

`recac` (Rewrite of Combined Autonomous Coding) is a comprehensive CLI tool that automates the setup, management, and deployment of containerized applications. It leverages AI agents to assist with coding tasks, manages development environments via Docker, and integrates with JIRA for project management.

## Features

- **🚀 Project Initialization**: Interactive wizard for creating new projects with predefined templates.
- **🐳 Container Management**: Auto-managed Docker environments for isolated development.
- **🤖 Multi-Agent System**: Built-in support for Gemini, OpenAI, and Ollama to perform autonomous coding tasks.
- **📋 Feature Tracking**: End-to-end feature lifecycle management (Spec -> Implementation -> Verification).
- **📊 Web Dashboard**: Real-time TUI and Web dashboard for monitoring progress.
- **🔗 Integrations**: JIRA for tickets, Slack/Discord for notifications, Git for version control.
- **🔄 Orchestration**: Pool and process work items from Jira or files using autonomous agents.

## Quick Start

### Prerequisites

- **Go 1.21+**
- **Docker** and **Docker Compose**
- **Git**

### Installation

```bash
# Clone the repository
git clone https://github.com/your-org/recac.git
cd recac

# Build the binary
go build -o recac ./cmd/recac

# Move to PATH (optional)
sudo mv recac /usr/local/bin/
```

### Configuration

Create a `config.yaml` in your project root or home directory (`$HOME/.recac.yaml`).

```yaml
# Core settings
debug: true
timeout: 300s
max_iterations: 20

# AI Provider (gemini, openai, ollama)
agent_provider: gemini
api_key: "your-api-key"
model: "gemini-pro"

# Docker settings
docker_image: "ubuntu:latest"
workspace_mount: "/workspace"

# Integrations (Optional)
jira_url: "https://your-domain.atlassian.net"
jira_email: "user@example.com"
jira_token: "api-token"
```

## Usage

### 1. Initialize a Project

Start a new project with the interactive wizard:

```bash
recac init
```

This will create the necessary directory structure, `app_spec.txt` for requirements, and `feature_list.json` for tracking.

### 2. Start a Session

Start the autonomous coding session. This spins up the Docker container and starts the AI agent loop.

```bash
recac start
```

The agent will read `app_spec.txt` and begin working on tasks.

### 3. Manage Features

Manage feature branches:

```bash
# Start a new feature
recac feature start "User Authentication"
```

### 4. Session Management

Recac allows you to manage multiple concurrent or background sessions.

- **List Sessions**: View all active and completed sessions.
  ```bash
  recac list
  ```

- **Check Status**: View detailed status of sessions, Docker environment, and configuration.
  ```bash
  recac status
  ```

- **Attach to Session**: Re-attach to a running session to view its live logs.
  ```bash
  recac attach <session-name>
  ```

### 5. Orchestration

The orchestrator pools work items (e.g., Jira tickets) and spawns agents to handle them.

```bash
# Run locally (uses Docker to spawn agents)
recac orchestrate --mode local --jira-label "recac-agent"

# Run in Kubernetes (spawns K8s Jobs)
recac orchestrate --mode k8s --namespace default
```

Common flags:
- `--mode`: `local` or `k8s` (default: local)
- `--poller`: `jira` or `file` (default: jira)
- `--jira-label`: Label to filter tickets by.
- `--interval`: Polling interval (default: 1m).

### 6. Troubleshooting & Maintenance

- **Pre-flight Check**: Check environment dependencies (Go, Docker, Config).
  ```bash
  recac check
  # Attempt to fix issues automatically
  recac check --fix
  ```

- **Signal Management**: Manage persistent project signals (stored in `.recac.db`).
  ```bash
  # Clear a specific signal (e.g., to reset project sign-off)
  recac signal clear PROJECT_SIGNED_OFF
  ```

- **Logs & Cleaning**:
  - `recac logs`: View agent and container logs.
  - `recac clean`: Remove temporary files and containers.

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
