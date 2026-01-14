package orchestrator

import (
	"context"
	"log/slog"
	"recac/internal/runner"
	"time"

	corev1 "k8s.io/api/core/v1"
)

// WorkItem represents a single task to be processed by an agent.
type WorkItem struct {
	ID          string
	Summary     string
	Description string
	RepoURL     string
	EnvVars     map[string]string // For secrets or other context
}

// Poller defines the interface for retrieving work items.
type Poller interface {
	Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error)
	UpdateStatus(ctx context.Context, item WorkItem, status string, message string) error
}

// Spawner defines the interface for creating and managing agent instances.
type Spawner interface {
	Spawn(ctx context.Context, item WorkItem) error
	Cleanup(ctx context.Context, item WorkItem) error
}

// K8sClient defines the interface for Kubernetes operations that the spawner needs.
// This allows for mocking in tests.
type K8sClient interface {
	CreateJob(ctx context.Context, namespace string, jobName string, image string, args []string, env []corev1.EnvVar, pullPolicy corev1.PullPolicy) error
	DeleteJob(ctx context.Context, namespace string, jobName string) error
	GetPodLogs(ctx context.Context, namespace, podName string, since *time.Time) (string, error)
	ListPods(ctx context.Context, namespace, labelSelector string) ([]corev1.Pod, error)
}

// ISessionManager defines the interface for session management operations.
// This allows for mocking in tests.
type ISessionManager interface {
	SaveSession(session *runner.SessionState) error
	LoadSession(name string) (*runner.SessionState, error)
	GetSessionGitDiffStat(name string) (string, error)
	StartSession(name string, command []string, workspace string) (*runner.SessionState, error)
}

// DockerClient abstraction for testing
type DockerClient interface {
	RunContainer(ctx context.Context, imageRef string, workspace string, extraBinds []string, ports []string, user string) (string, error)
	Exec(ctx context.Context, containerID string, cmd []string) (string, error)
	StopContainer(ctx context.Context, containerID string) error
}

// IGitClient defines the interface for git operations.
type IGitClient interface {
	Clone(ctx context.Context, repoURL, path string) error
	CurrentCommitSHA(path string) (string, error)
}
