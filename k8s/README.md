# Kubernetes Manifests for Jira Credentials

This directory contains Kubernetes manifests for managing Jira API credentials securely.

## Jira Secrets

The `jira-secrets.yaml` file defines a Kubernetes Secret that stores Jira credentials:

- `username`: Jira username/email
- `api-key`: Jira API token
- `base-url`: Jira instance base URL

### Applying the Secret

1. First, base64 encode your credentials:
