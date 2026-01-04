# Specification: Kubernetes Job Support for Agents

## 1. Overview

This specification details the addition of Kubernetes support to `recac`, allowing agents to run within Kubernetes Jobs instead of local Docker containers. This enables scalability, better resource management, and execution in remote environments.

Recac repository:
https://github.com/process-failed-successfully/recac

## 2. Architecture

### 2.1 Interface Abstraction

The current `DockerClient` interface in `internal/runner/docker_interface.go` is tightly coupled to Docker concepts. It will be renamed to `ContainerEngine` (or similar generic name) and generalized.

```go
type ContainerEngine interface {
    // Lifecycle
    Init(ctx context.Context) error // Check connectivity (Daemon/Cluster)
    StartEnvironment(ctx context.Context, imageRef string, workspace string, envVars map[string]string) (string, error) // Returns ID
    StopEnvironment(ctx context.Context, id string) error

    // Execution
    Exec(ctx context.Context, id string, cmd []string) (string, error)
    ExecAsUser(ctx context.Context, id string, user string, cmd []string) (string, error)

    // Image/Artifact Management
    EnsureImage(ctx context.Context, imageRef string) error // Pulls or verifies existence
    BuildImage(ctx context.Context, opts BuildOptions) (string, error) // Docker-specific mostly, K8s might skip or use Kaniko/BuildKit?
                                                                       // For V1, K8s implementation might assume pre-built images or fail on build.
}
```

### 2.2 Implementations

- `internal/docker`: Existing Docker implementation.
- `internal/kubernetes`: New package implementing the interface using `client-go`.

## 3. Kubernetes Implementation Details

### 3.1 Resource Type: Job vs Pod

We will use `batch/v1 Job` for spawning the agent environment.

- **Why Job?** Jobs provide restart policies (`OnFailure`) and clear lifecycle management. If the node dies, the Job controller can reschedule it (though we lose ephemeral state, `recac` should be robust to this).
- **Keep-Alive**: The Job's container command will be `sleep infinity` (or a lightweight wait loop) to allow the `recac` runner to `Exec` commands into it interactively.

### 3.2 Workspace Persistence

- **Local Execution**: When `recac` runs locally and targets a remote cluster, we cannot "bind mount" the local workspace easily.

  - _Solution A (MVP)_: Assume `recac` runs **inside** the cluster (e.g., as a pod itself) and shares a PersistentVolumeClaim (PVC) with the agent Job.
  - _Solution B (Remote)_: Use `kubectl cp` (via client-go `SPDY` executor) to sync files up/down. This is slow for large repos.
  - _Solution C (Git)_: The agent Job clones the repo from the remote origin. `recac` manages the agent via Git ops (pushing changes).

  **Decision for V1**:

  1.  If `KUBECONFIG` points to a local cluster (e.g., Minikube, Docker Desktop), try `hostPath`.
  2.  Standard mode: `recac` expects the workspace to be available via a PVC. The specification assumes the user provides a `pvc_name` in config.
  3.  Future: `recac` uploads the current context to a PVC or ephemeral volume.

### 3.3 Execution Flow

1.  **Start**: `recac` creates a Job.
    - Name: `recac-agent-<session-id>`
    - Image: `ghcr.io/.../recac-agent`
    - VolumeMount: `workspace-pvc:/workspace`
2.  **Wait**: Poll for Pod status `Running`.
3.  **Exec**: Use `client-go/tools/remotecommand` to execute shell commands (`/bin/bash -c ...`) inside the Pod.
4.  **Stop**: Delete the Job (and associated Pods).

## 4. Configuration

Additions to `config.yaml`:

```yaml
runner:
  backend: "kubernetes" # or "docker"

kubernetes:
  kubeconfig: "~/.kube/config" # optional, defaults to standard resolution
  context: "my-cluster" # optional
  namespace: "recac-agents" # default: default
  pvc_name: "recac-workspace" # Required for persistence
  resources:
    requests:
      cpu: "500m"
      memory: "1Gi"
    limits:
      cpu: "2000m"
      memory: "4Gi"
  service_account: "recac-agent-sa" # optional
```

## 5. Scaling and Best Practices

### 5.1 Resource Isolation

- **Namespace**: Agents should run in a dedicated namespace (`recac-agents`) to avoid clutter and enforce quotas.
- **Quotas**: Use `ResourceQuota` to limit total CPU/Memory consumed by all agents.

### 5.2 Security

- **ServiceAccount**: The agent pod should run with a ServiceAccount that has **no** permissions (or minimal) to prevent the agent from messing with the cluster.
- **NetworkPolicy**: Restrict egress. Agents usually need Internet access (pip/go get), but should not access internal cluster services (Kube API, internal DBs).

### 5.3 Concurrency

- `recac` supports multi-agent sessions. In K8s, this maps to multiple Jobs.
- Scaling is handled by the K8s scheduler. If the cluster is full, Jobs stay `Pending`. `recac` must handle timeouts waiting for `Running` state.

## 6. Migration Plan

1.  Refactor `internal/runner` to use the `ContainerEngine` interface.
2.  Move `DockerClient` implementation details to `internal/docker`.
3.  Implement `internal/kubernetes`.
4.  Update `Session` to load the correct engine based on config.
