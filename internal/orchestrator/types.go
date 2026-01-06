package orchestrator

import (
	"context"
	"time"
)

// WorkItem represents a unit of work (e.g., a Jira ticket)
type WorkItem struct {
	ID          string // Unique ID (e.g. Jira Key)
	Summary     string
	Description string
	RepoURL     string // URL to clone
	// EnvVars to pass to the agent
	EnvVars map[string]string
}

// Poller defines the interface for discovering work
type Poller interface {
	// Poll returns a list of new work items
	Poll(ctx context.Context) ([]WorkItem, error)
	// Claim marks a work item as "in progress" so others don't pick it up
	Claim(ctx context.Context, item WorkItem) error
	// UpdateStatus updates the status of the work item (e.g. Done, Failed)
	UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error
}

// Spawner defines the interface for running agents
type Spawner interface {
	// Spawn starts an agent for the given work item
	Spawn(ctx context.Context, item WorkItem) error
	// Cleanup removes resources associated with the work item
	Cleanup(ctx context.Context, item WorkItem) error
}

// Orchestrator manages the lifecycle of work items
type Orchestrator struct {
	Poller       Poller
	Spawner      Spawner
	PollInterval time.Duration
}

func New(poller Poller, spawner Spawner, interval time.Duration) *Orchestrator {
	return &Orchestrator{
		Poller:       poller,
		Spawner:      spawner,
		PollInterval: interval,
	}
}
