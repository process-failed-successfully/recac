# RECAC End-to-End (E2E) Testing

RECAC features a sophisticated E2E testing framework that validates the entire system flow—from task polling to code generation and final verification—using real-world scenarios.

## Overview

The E2E suite simulates the interaction between the Orchestrator, the Agent, and external services (Jira, Git, AI Providers). It ensures that architectural changes or refactors don't break the core "Poll-Spawn-Verify" loop.

## The E2E Runner

The runner is located in [`e2e/runner`](runner/main.go). It handles the setup of a temporary test environment, including:

- Mocking or using a dedicated Jira instance.
- Initializing a Git repository with base code.
- Deploying the Orchestrator (locally or in K8s).
- Monitoring and verifying the Agent's output.

### Running Tests Locally

To verify logic without Kubernetes overhead:

```bash
go run ./e2e/runner -local -scenario prime-python
```

This runs the Orchestrator as a local process and spawns Agents as Docker containers.

### Running Tests in Kubernetes (Simulation)

To simulate a production deployment:

```bash
make ci-simulate
```

This command:

1. Builds the latest Docker image.
2. Pushes it to a local/test registry.
3. Deploys the system via Helm.
4. Runs the E2E runner to verify the end-to-end flow within the cluster.

## Scenarios

Scenarios are defined in [`pkg/e2e/scenarios`](../pkg/e2e/scenarios). Each scenario defines:

1. **Setup**: The initial state of the repository and the Jira ticket content.
2. **Execution**: The task instructions for the agent.
3. **Verification**: Custom logic to validate the agent's work (e.g., checking for specific files, output values, or commit structures).

### Available Scenarios

- **`prime-python`**: Tasks the agent with creating a Python script to calculate prime numbers. Verifies the JSON output for accuracy (exactly 1229 primes < 10,000).

## Adding a New Scenario

1. Create a new file in `pkg/e2e/scenarios/`.
2. Implement the `Scenario` interface (Setup, Task, Verify).
3. Register the scenario in the `ScenarioRegistry` (usually via `init()` function).
