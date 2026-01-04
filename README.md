# Jira Polling Logic

This project implements the polling logic in the orchestrator to periodically query Jira for tickets in a 'Ready' state or with specific labels (e.g., 'recac-agent').

## Features

<<<<<<< Updated upstream
- Configurable polling interval via environment variables
- Secure management of Jira API credentials using Kubernetes Secrets
- Filtering of tickets by state and labels
- Error handling for Jira API failures
- Unit tests for polling logic
=======
- ✅ Kubernetes Job template with proper structure (apiVersion, kind, metadata, spec)
- InitContainer for git cloning
- Main container for executing the recac agent
- Local Kubernetes testing support
>>>>>>> Stashed changes

## Setup

<<<<<<< Updated upstream
1. Run the initialization script:
=======
- **3 replicas** for redundancy
- **Pod anti-affinity** to ensure replicas run on different nodes
- **Resource requests/limits** for proper resource allocation
- **Leader election labels** for proper identification

### Deployment Structure

## Implementation Status

### Completed Features
- ✅ Leader Election Implementation
  - Implemented using client-go leaderelection package
  - Runs in background goroutine
  - Proper configuration setup
  - Unit tests included

### Usage

To use the leader election in your application:

## Failover Testing

The orchestrator deployment is configured for high availability with failover testing:

- **3 replicas** for redundancy
- **Pod anti-affinity** to ensure replicas run on different nodes
- **Resource requests/limits** for proper resource allocation
- **Leader election labels** for proper identification

### Running Failover Tests

To test the failover mechanism:
- Enhanced logging for leader election events
>>>>>>> Stashed changes
