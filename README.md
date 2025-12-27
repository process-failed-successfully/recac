# RECAC: Combined Autonomous Coding (Go Refactor)

## Overview

RECAC is a premium, type-safe, and high-performance autonomous coding framework re-implemented in Go. It provides a world-class CLI/TUI experience for managing autonomous coding sessions. RECAC enables developers to automate coding tasks by leveraging AI agents that can read specifications, plan implementations, execute code changes, and manage the entire development workflow.

## Features

- **Interactive TUI**: Beautiful terminal user interface with dashboard, sprint board, and project wizard
- **Multi-Agent Support**: Works with Gemini and OpenAI models
- **Docker Integration**: Isolated execution environments for safe code execution
- **Jira Integration**: Automatic ticket management and status updates
- **Worker Pool**: Parallel task execution with configurable concurrency
- **State Management**: Persistent agent state and memory across sessions
- **Token Management**: Intelligent context window management with automatic truncation
- **Comprehensive Testing**: Full test coverage with mocks and integration tests
- **Error Handling**: Graceful error recovery and panic handling
- **Telemetry**: Structured logging and Prometheus metrics

## Installation & Setup

### Prerequisites

- **Go 1.21+**: Required for building and running RECAC
- **Docker**: Required for containerized code execution (optional for mock mode)

### Quick Start

1. **Clone the repository** (if not already available):
   ```bash
   git clone <repository-url>
   cd recac
   ```

2. **Install Go** (if not already installed):
   ```bash
   ./install_go.sh
   export PATH=$PATH:$HOME/go-dist/go/bin
   ```

3. **Build the application**:
   ```bash
   make build
   # Or manually:
   go build -o recac-app ./cmd/recac
   ```

4. **Verify installation**:
   ```bash
   ./recac-app version
   # Should output: v0.1.0
   ```

5. **Check Docker** (optional, for real execution):
   ```bash
   ./recac-app check-docker
   ```

## Usage Examples

### Starting a Session

**Interactive Mode** (with project wizard):
```bash
./recac-app start
```
This launches an interactive wizard to configure your project path and settings.

**Direct Mode** (skip wizard):
```bash
./recac-app start --path /path/to/project
```

**Mock Mode** (for testing without Docker):
```bash
./recac-app start --mock
./recac-app start --mock-docker
```

**Verbose Logging**:
```bash
./recac-app start --verbose
```

### Sprint Mode (Parallel Execution)

Run multiple agents in parallel to handle independent tasks:

```bash
# Default: 3 workers
./recac-app sprint

# Custom worker count
./recac-app sprint --workers 5

# With Slack notifications
./recac-app sprint --slack-webhook https://hooks.slack.com/services/YOUR/WEBHOOK/URL
```

### Project Initialization

Initialize a new project structure from a specification:

```bash
# Basic initialization
./recac-app init-project --spec app_spec.txt

# Force overwrite existing files
./recac-app init-project --spec app_spec.txt --force

# Use mock agent for testing
./recac-app init-project --spec app_spec.txt --mock-agent
```

### Docker Image Building

Build Docker images for your project:

```bash
# Build with default Dockerfile
./recac-app build --context ./my-project --tag myapp:latest

# Custom Dockerfile path
./recac-app build --context ./my-project --dockerfile ./Dockerfile.custom --tag myapp:latest

# Build without cache
./recac-app build --context ./my-project --tag myapp:latest --no-cache
```

### Cleanup

Remove temporary files created during sessions:

```bash
./recac-app clean
```

This removes files listed in `temp_files.txt` from the workspace.

### Feature Management

Manage features in your project:

```bash
./recac-app feature --help
```

### Getting Help

```bash
# General help
./recac-app --help

# Command-specific help
./recac-app start --help
./recac-app sprint --help
```

## Configuration Guide

RECAC uses YAML configuration files and environment variables. Configuration is loaded in this order:
1. Command-line flags
2. Environment variables (prefixed with `RECAC_`)
3. Config file (`config.yaml` in current directory or `$HOME/.recac.yaml`)

### Configuration File Structure

Create a `config.yaml` file in your project root:

```yaml
# Debug mode
debug: true

# Verbose logging
verbose: false

# Timeout settings (in seconds or duration strings like "30s", "5m")
timeout: 300
agent_timeout: 60
docker_timeout: 120
bash_timeout: 30

# Agent settings
max_iterations: 10
max_agents: 5

# Worker pool settings
workers: 3

# Port settings (1-65535)
port: 8080
metrics_port: 9090

# Manager review frequency
manager_frequency: 10

# Agent provider settings
agent:
  provider: "gemini"  # or "openai"
  api_key: "${GEMINI_API_KEY}"  # Use environment variables for secrets
  model: "gemini-pro"
  max_tokens: 32000

# Docker settings
docker:
  image: "golang:1.21"
  workspace_mount: "/workspace"

# Jira settings
jira:
  base_url: "https://your-domain.atlassian.net"
  username: "${JIRA_USERNAME}"
  api_token: "${JIRA_API_TOKEN}"

# Slack notifications
slack:
  webhook_url: "${SLACK_WEBHOOK_URL}"
```

### Environment Variables

All configuration values can be overridden using environment variables with the `RECAC_` prefix:

```bash
export RECAC_DEBUG=true
export RECAC_WORKERS=5
export RECAC_AGENT_PROVIDER=openai
export RECAC_AGENT_API_KEY=your-api-key
```

### Configuration Validation

RECAC validates configuration values on startup:
- **Timeouts**: Must be positive values
- **Ports**: Must be between 1 and 65535
- **Workers/Agents/Iterations**: Must be positive integers
- **Invalid values**: Application exits with error message

Example error:
```bash
$ ./recac-app start
Error: configuration validation failed:
  timeout must be positive, got: -5
  port must be between 1 and 65535, got: 99999
```

## Architecture Overview

RECAC follows a modular architecture with clear separation of concerns:

```
recac/
├── cmd/recac/          # CLI commands (start, sprint, init-project, etc.)
├── internal/
│   ├── agent/          # AI agent clients (Gemini, OpenAI)
│   │   ├── interface.go    # Agent interface
│   │   ├── gemini.go       # Gemini implementation
│   │   ├── openai.go       # OpenAI implementation
│   │   ├── state.go        # State persistence
│   │   └── tokens.go       # Token counting & management
│   ├── config/         # Configuration validation
│   ├── docker/         # Docker client & container management
│   ├── jira/           # Jira API client
│   ├── notify/         # Notification system (Slack)
│   ├── runner/         # Core execution engine
│   │   ├── session.go      # Session management
│   │   ├── workflow.go     # Workflow orchestration
│   │   ├── planner.go      # Task planning
│   │   ├── pool.go         # Worker pool
│   │   └── qa.go           # Quality assurance
│   ├── telemetry/      # Logging & metrics
│   ├── ui/             # Terminal UI components
│   │   ├── dashboard.go    # Main dashboard
│   │   ├── sprint_board.go # Sprint board view
│   │   └── wizard.go       # Project wizard
│   └── pkg/git/        # Git operations
└── config.yaml          # Configuration file
```

### Key Components

**Agent Package** (`internal/agent/`):
- Provides unified interface for multiple AI providers
- Manages conversation state and memory
- Handles token counting and context window management
- Implements retry logic with exponential backoff

**Runner Package** (`internal/runner/`):
- Orchestrates the complete development workflow
- Manages Docker containers for isolated execution
- Coordinates agent interactions and code execution
- Implements worker pool for parallel task processing

**UI Package** (`internal/ui/`):
- Bubble Tea-based terminal user interface
- Dashboard for monitoring agent activity
- Sprint board for task management
- Interactive wizard for project setup

**Docker Package** (`internal/docker/`):
- Abstracts Docker operations for testability
- Handles container lifecycle (create, exec, stop)
- Manages workspace mounting
- Supports image building

## Development Setup

### Building from Source

```bash
# Clone repository
git clone <repository-url>
cd recac

# Install dependencies
go mod download

# Build
make build
# Or: go build -o recac-app ./cmd/recac

# Run tests
go test ./...

# Run with race detector
go test -race ./...
```

### Running Tests

```bash
# All tests
go test ./...

# Specific package
go test ./internal/agent

# Verbose output
go test -v ./internal/runner

# With coverage
go test -cover ./...

# Integration test
go test ./internal/runner -v -run TestEndToEndHelloWorld
```

### Code Quality

```bash
# Format code
go fmt ./...

# Vet code
go vet ./...

# Lint (if golangci-lint is installed)
golangci-lint run
```

### Project Structure

The project follows standard Go project layout:
- `cmd/`: Application entry points
- `internal/`: Private application code (not importable by other projects)
- `pkg/`: Public libraries (if any)
- `config.yaml`: Configuration file
- `feature_list.json`: Feature test specifications

## Troubleshooting

### Common Issues

**1. "go: command not found"**
```bash
# Install Go using the provided script
./install_go.sh
export PATH=$PATH:$HOME/go-dist/go/bin
```

**2. "docker daemon is not reachable"**
```bash
# Check Docker is running
docker ps

# If permission denied, add user to docker group (Linux)
sudo usermod -aG docker $USER
# Then log out and back in

# Or use mock mode for testing
./recac-app start --mock-docker
```

**3. "configuration validation failed"**
- Check your `config.yaml` for invalid values
- Ensure timeouts are positive
- Verify ports are in range 1-65535
- Check that worker/agent counts are positive

**4. "panic: runtime error"**
- RECAC includes panic recovery and will attempt graceful shutdown
- Check logs for stack traces
- Report issues with full error output

**5. TUI not displaying correctly**
- Ensure terminal supports ANSI colors
- Try resizing terminal window
- Check terminal emulator compatibility (tested with: xterm, gnome-terminal, iTerm2)

**6. Agent API errors**
- Verify API keys are set correctly in environment variables
- Check network connectivity
- Review retry logic in agent package (exponential backoff)
- Enable verbose logging: `./recac-app start --verbose`

**7. Tests failing**
- Ensure Go 1.21+ is installed
- Run `go mod download` to fetch dependencies
- Check that test files are in correct locations
- Verify mock implementations are up to date

### Debug Mode

Enable debug/verbose logging for detailed information:

```bash
# Via command line
./recac-app start --verbose

# Via config file
echo "verbose: true" >> config.yaml

# Via environment variable
export RECAC_VERBOSE=true
./recac-app start
```

### Getting Help

- Check command help: `./recac-app --help` or `./recac-app <command> --help`
- Review `app_spec.txt` for detailed requirements
- Check `feature_list.json` for test specifications
- Review logs in verbose mode

### Reporting Issues

When reporting issues, please include:
1. RECAC version: `./recac-app version`
2. Go version: `go version`
3. Docker version: `docker --version`
4. Configuration (sanitized): `cat config.yaml`
5. Verbose logs: `./recac-app <command> --verbose`
6. Error messages and stack traces

## Specification

For detailed requirements and feature specifications, see:
- `app_spec.txt`: Full application specification
- `feature_list.json`: Complete feature test list with verification steps

## License

[Add license information here]

## Contributing

[Add contributing guidelines here]
