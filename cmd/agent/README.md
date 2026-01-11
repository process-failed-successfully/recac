# RECAC Agent

The RECAC Agent is the execution engine of the system. It is a specialized autonomous agent capable of analyzing requirements, writing code, running tests, and managing Git branches to deliver features.

## Key Features

- **Autonomous Coding**: Iterative loop of planning, execution, and verification.
- **Git Native**: Manages feature branches and pushes progress automatically.
- **Docker/k8s Aware**: Can run inside a container or as a standalone process (local mode).
- **Quality Focused**: Built-in QA and Manager review roles ensure code quality before delivery.

## Usage

Agents are typically spawned by the [Orchestrator](../orchestrator/README.md), but they can be run manually for debugging or direct tasks.

```bash
./bin/recac-agent [flags]
```

### Essential Flags

| Flag               | Default | Description                                           |
| ------------------ | ------- | ----------------------------------------------------- |
| `--jira`           | -       | Jira Ticket ID (e.g., `RD-123`) to load context from. |
| `--repo-url`       | -       | Repository URL to clone (overrides Jira).             |
| `--summary`        | -       | Task summary (required for direct tasks).             |
| `--description`    | -       | Detailed task instructions.                           |
| `--path`           | `.`     | Working directory for the workspace.                  |
| `--max-iterations` | `20`    | Fail-safe limit for the agent loop.                   |
| `--provider`       | -       | AI provider (overrides config).                       |
| `--model`          | -       | AI model (overrides config).                          |

## Environment Variables

The agent requires specific environment variables for AI providers and integrations:

- `RECAC_PROVIDER`: `gemini`, `openai`, `openrouter`, or `ollama`.
- `RECAC_MODEL`: The specific model name.
- `API_KEY`: API key for the selected provider.
- `GITHUB_TOKEN`: Required for pushing to GitHub repositories.
- `RECAC_DB_URL`: Connection string for project persistence (PostgreSQL/SQLite).

## Signals & Lifecycle

The agent uses a "Signal" system to coordinate transitions between roles (Agent -> QA -> Manager).

- `COMPLETED`: Created by the Agent when they believe the task is done.
- `QA_PASSED`: Created by the QA agent after verifying the implementation.
- `PROJECT_SIGNED_OFF`: Created by the Manager agent (or human) for final finalization.
- `TRIGGER_QA`: Manually force a QA round.

Signals are stored in the project's database. Setting `PROJECT_SIGNED_OFF` to `true` will cause the agent to perform a final merge and exit.

## Running Locally

To run an agent against a local folder without cloning:

```bash
./bin/recac-agent --path ./my-project --summary "Fix bug in login.py" --allow-dirty
```
