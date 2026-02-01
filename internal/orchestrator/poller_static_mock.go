package orchestrator

import (
	"context"
	"log/slog"
)

// StaticMockPoller implements Poller interface for E2E testing
type StaticMockPoller struct {
	Items []WorkItem
}

func NewStaticMockPoller() *StaticMockPoller {
	return &StaticMockPoller{
		Items: []WorkItem{
			{
				ID:          "MOCK-1",
				Summary:     "Mock Task",
				Description: "Repo: https://github.com/example/repo\n\nTask Description",
				RepoURL:     "https://github.com/example/repo",
				EnvVars: map[string]string{
					"JIRA_TICKET": "MOCK-1",
				},
			},
		},
	}
}

func (p *StaticMockPoller) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	if len(p.Items) > 0 {
		items := p.Items
		p.Items = nil // Return items once
		return items, nil
	}
	return nil, nil
}

func (p *StaticMockPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	// No-op
	return nil
}
