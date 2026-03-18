package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"time"

	"recac/internal/jira"
	"recac/internal/runner"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

// WorkItem represents a unit of work to be processed, e.g., a Jira ticket.
type WorkItem struct {
	ID               string            `json:"id"`
	Summary          string            `json:"summary"`
	Description      string            `json:"description"`
	RepoURL          string            `json:"repo_url"` // Repo to clone
	EnvVars          map[string]string `json:"env_vars,omitempty"`
	DependsOn        []string          `json:"depends_on,omitempty"`
	Priority         int               `json:"priority,omitempty"`
	Tags             []string          `json:"tags,omitempty"`
	RunAfter         time.Time         `json:"run_after,omitempty"`
	Delay            time.Duration     `json:"delay,omitempty"`
	Timeout          time.Duration     `json:"timeout,omitempty"`
	ConcurrencyGroup string            `json:"concurrency_group,omitempty"`
	CancelInProgress bool              `json:"cancel_in_progress,omitempty"`
	AgentProvider    string            `json:"agent_provider,omitempty"`
	AgentModel       string            `json:"agent_model,omitempty"`
	Hold             bool              `json:"hold,omitempty"`
	MaxRetries       *int              `json:"max_retries,omitempty"`
	RunCondition     string            `json:"run_condition,omitempty" yaml:"run_condition"`
}

// Poller defines the interface for polling for work items.
type Poller interface {
	Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error)
	UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error
	Ping(ctx context.Context) error
}

// Spawner defines the interface for spawning an agent to handle a work item.
type Spawner interface {
	Spawn(ctx context.Context, item WorkItem) error
	Cleanup(ctx context.Context, item WorkItem) error
	Cancel(ctx context.Context, jobID string) error
	GetLogs(ctx context.Context, jobID string) (io.ReadCloser, error)
	Ping(ctx context.Context) error
}

// Notifier defines the interface for sending notifications.
type Notifier interface {
	Notify(ctx context.Context, eventType string, message string, threadStateStr string) (string, error)
}

// JiraClient defines the interface for a Jira client, created for mocking purposes.
// It mirrors the methods of jira.Client used by JiraPoller.
type JiraClient interface {
	SearchIssues(ctx context.Context, jql string) ([]map[string]interface{}, error)
	GetBlockers(issue map[string]interface{}) []string
	GetBlockerKeys(issue map[string]interface{}) []string
	ParseDescription(issue map[string]interface{}) string
	AddComment(ctx context.Context, issueID string, comment string) error
	SmartTransition(ctx context.Context, issueID string, status string) error
}

// Statically assert that the real client implements our interface.
var _ JiraClient = (*jira.Client)(nil)

// DockerClient defines the interface for Docker operations, created for mocking.
type DockerClient interface {
	RunContainer(ctx context.Context, image string, workspace string, binds []string, env []string, cmd []string, user string) (string, error)
	RunContainerWithLabels(ctx context.Context, image string, workspace string, binds []string, env []string, cmd []string, user string, labels map[string]string) (string, error)
	StopContainer(ctx context.Context, containerID string) error
	Exec(ctx context.Context, containerID string, cmd []string) (string, error)
	ListContainers(ctx context.Context, options container.ListOptions) ([]types.Container, error)
	RemoveContainer(ctx context.Context, containerID string, force bool) error
	ContainerLogs(ctx context.Context, containerID string) (io.ReadCloser, error)
	WaitContainer(ctx context.Context, containerID string) (int64, error)
	ImageExists(ctx context.Context, tag string) (bool, error)
	PullImage(ctx context.Context, imageRef string) error
}

// ISessionManager defines the interface for session management, created for mocking.
type ISessionManager interface {
	SaveSession(session *runner.SessionState) error
	LoadSession(name string) (*runner.SessionState, error)
}

// IGitClient defines the interface for Git operations, created for mocking.
type IGitClient interface {
	Clone(ctx context.Context, repoURL, destPath string) error
	CurrentCommitSHA(repoPath string) (string, error)
}
