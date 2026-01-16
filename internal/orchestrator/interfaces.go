package orchestrator

import (
	"context"
	"log/slog"
	"recac/internal/jira"
	"recac/internal/runner"
)

// WorkItem represents a unit of work to be processed, e.g., a Jira ticket.
type WorkItem struct {
	ID          string
	Summary     string
	Description string
	RepoURL     string // Repo to clone
	EnvVars     map[string]string
}

// Poller defines the interface for polling for work items.
type Poller interface {
	Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error)
	UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error
}

// Spawner defines the interface for spawning an agent to handle a work item.
type Spawner interface {
	Spawn(ctx context.Context, item WorkItem) error
	Cleanup(ctx context.Context, item WorkItem) error
}

// JiraClient defines the interface for a Jira client, created for mocking purposes.
// It mirrors the methods of jira.Client used by JiraPoller.
type JiraClient interface {
	SearchIssues(ctx context.Context, jql string) ([]map[string]interface{}, error)
	GetBlockers(issue map[string]interface{}) []string
	ParseDescription(issue map[string]interface{}) string
	AddComment(ctx context.Context, issueID string, comment string) error
	SmartTransition(ctx context.Context, issueID string, status string) error
}

// Statically assert that the real client implements our interface.
var _ JiraClient = (*jira.Client)(nil)

// DockerClient defines the interface for Docker operations, created for mocking.
type DockerClient interface {
	RunContainer(ctx context.Context, image string, workspace string, binds []string, env []string, user string) (string, error)
	StopContainer(ctx context.Context, containerID string) error
	Exec(ctx context.Context, containerID string, cmd []string) (string, error)
	ImageExistsLocally(ctx context.Context, imageName string) (bool, error)
	PullImage(ctx context.Context, imageName string) error
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
