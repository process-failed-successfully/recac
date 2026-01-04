# Jira Polling Logic

This project implements the polling logic in the orchestrator to periodically query Jira for tickets in a 'Ready' state or with specific labels (e.g., 'recac-agent').

## Features

- Configurable polling interval via environment variables
- Secure management of Jira API credentials
- Filtering of tickets by state and labels
- Error handling for Jira API failures
- Unit tests for polling logic

## Setup

1. Run the initialization script:
