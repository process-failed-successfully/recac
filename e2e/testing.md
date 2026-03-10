# End-to-End (E2E) Testing Guide

This guide describes how to run End-to-End tests for `recac`. There are two main categories of E2E tests:

1.  **Automated Local E2E**: Runs the Orchestrator with a local Docker backend.
2.  **Kubernetes E2E**: Manual verification steps for running the system in a K8s cluster.

## Prerequisites

All E2E tests require valid credentials for Jira and (optionally) LLM providers/GitHub.
Ensure you have a `.env` file in the project root with the following:

```bash
# Jira Credentials
JIRA_URL="https://your-domain.atlassian.net"
JIRA_EMAIL="user@example.com"
JIRA_API_TOKEN="your-jira-token"
JIRA_PROJECT_KEY="PROJ"

# Agent Credentials (Optional for Poller tests, Required for Full Flow)
OPENROUTER_API_KEY="..."
GITHUB_API_KEY="..." # For pushing code
```

## 1. Automated Local E2E Tests

These tests use the Go testing framework to spin up a local Orchestrator and Docker containers.

**Location**: `tests/e2e/`

**Command**:

```bash
# Run all E2E tests
go test -v -tags=e2e ./tests/e2e/...

# Run a specific test (e.g. Full Flow)
go test -v -tags=e2e -run TestOrchestrator_FullFlow_E2E ./tests/e2e/...
```

**What it tests**:

- **Jira Polling**: Verifies the orchestrator can find and claim tickets.
- **Docker Spawning**: Verifies the orchestrator can spawn a local Docker container for an agent.
- **Agent Execution**: (Full Flow) Verifies the agent generates code and pushes it to a Git repository.

## 2. Kubernetes E2E Verification

To verify the system in a Kubernetes environment (validating split-filesystem fixes, service accounts, and K8s Job spawning), follow this workflow.

### Step 1: Build & Push Image

Build an image accessible to your K8s cluster (e.g., using `ttl.sh` for ephemeral testing).

```bash
export TAG="recac-e2e-$(date +%s):1h"
docker build -t ttl.sh/$TAG -f Dockerfile --target production .
docker push ttl.sh/$TAG
echo "Image: ttl.sh/$TAG"
```

### Step 2: Configure Environment

Ensure your `env` variables are loaded or available to the Orchestrator.

### Step 3: Run Orchestrator in K8s Mode

You can run the orchestrator locally (pointing to K8s) or deploy it. To run locally but targeting K8s:

```bash
# Requires valid KUBECONFIG and .env
# Enable K8s mode via flag, and point to the image built above
go run ./cmd/recac orchestrate \
  --mode k8s \
  --image ttl.sh/$TAG \
  --jira-label recac-agent \
  --agent-provider openrouter \
  --agent-model anthropic/claude-3.5-sonnet
```

### Step 4: Verify

1.  **Create a Jira Ticket**: Create a ticket with label `recac-agent`.
2.  **Monitor Logs**: The orchestrator should see the ticket and spawn a Kubernetes Job.
    ```bash
    kubectl get jobs
    kubectl logs -f job/recac-agent-<ticket-id>
    ```
3.  **Verify Git Push**: Check the target repository (defined in the ticket) for a new branch `agent/<ticket-id>` containing the generated code.

> **Note**: For Kubernetes execution, the agent uses **Local Agent Mode** (bypassing nested Docker) if it detects it is running inside a Pod. This ensures file system consistency.
