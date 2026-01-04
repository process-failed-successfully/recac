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

1. Run the initialization script:
