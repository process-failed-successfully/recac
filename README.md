<<<<<<< Updated upstream
# Recac Orchestrator Deployment

<<<<<<< Updated upstream
This project contains the Kubernetes deployment configuration for the recac orchestrator service.
=======
# Agent Job Template

This project implements a Kubernetes Job template for agents, including initialization steps and command execution.
>>>>>>> Stashed changes
=======
This project implements high availability for the orchestrator using Kubernetes leader election and lease API.

## Features

### Single Active Instance
Ensures only one orchestrator instance is active at any time using Kubernetes Lease API for leader election.

### Multi-Replica Support
Supports multiple replicas of the orchestrator for high availability.

### Leader Election
Implements leader election using Kubernetes Lease API to coordinate between multiple instances.

## Usage
>>>>>>> Stashed changes

1. Deploy the orchestrator with multiple replicas
2. The leader election mechanism will ensure only one instance is active
3. If the leader fails, another instance will automatically take over

<<<<<<< Updated upstream
<<<<<<< Updated upstream
=======
- ✅ Kubernetes Job YAML template with proper spec
- ✅ InitContainer or entrypoint performs git clone operation
- ✅ YAML syntax validation

## Setup

>>>>>>> Stashed changes
Run the following command to set up the development environment:
=======
## Development

### Building
>>>>>>> Stashed changes
