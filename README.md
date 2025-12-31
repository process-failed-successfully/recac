# recac - Rewrite of Combined Autonomous Coding

`recac` (Rewrite of Combined Autonomous Coding) is a comprehensive CLI tool that automates the setup, management, and deployment of containerized applications. It leverages AI agents to assist with coding tasks, manages development environments via Docker, and integrates with JIRA for project management.

## Features

- **🚀 Project Initialization**: Interactive wizard for creating new projects with predefined templates.
- **🐳 Container Management**: Auto-managed Docker environments for isolated development.
- **🤖 Multi-Agent System**: Built-in support for Gemini, OpenAI, and Ollama to perform autonomous coding tasks.
- **📋 Feature Tracking**: End-to-end feature lifecycle management (Spec -> Implementation -> Verification).
- **📊 Web Dashboard**: Real-time TUI and Web dashboard for monitoring progress.
- **🔗 Integrations**: JIRA for tickets, Slack/Discord for notifications, Git for version control.

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
go build -o recac cmd/recac/main.go

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

Track and verify features directly from the CLI:

```bash
# List all features
recac feature list

# Add a new feature
recac feature add "User Authentication" --desc "Implement login via JWT"

# Check status
recac feature status
```

### 4. Other Commands

- `recac stop`: Stop the running development container.
- `recac logs`: View agent and container logs.
- `recac clean`: Remove temporary files and containers.
- `recac build`: Build the project (if applicable).
- `recac jira sync`: Sync local features with JIRA issues.

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

## Architecture

`recac` is built with a modular architecture:

- **cmd/**: CLI entry points (Cobra).
- **internal/agent/**: AI provider abstractions.
- **internal/runner/**: The core workflow loop and state management.
- **internal/docker/**: Docker API client wrappers.
- **internal/ui/**: TUI components (Bubble Tea).

## License

MIT
