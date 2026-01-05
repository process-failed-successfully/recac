# Agent Documentation

## Overview

Agents are lightweight workloads managed by the Kubernetes Operator. They perform specific tasks and report status back to the operator.

## Features

- **Automatic Registration**: Agents automatically register with the operator
- **Health Reporting**: Continuous health status updates
- **Task Execution**: Execute assigned tasks and report results
- **Self-Update**: Automatic updates when new versions are available

## Deployment

### Prerequisites

- Kubernetes cluster with operator deployed
- kubectl configured to access the cluster

### Installation

1. Deploy an agent:
