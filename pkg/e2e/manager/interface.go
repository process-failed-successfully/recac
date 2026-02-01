package manager

import "context"

type TicketManager interface {
	GenerateScenario(ctx context.Context, scenarioName, repoURL, provider, model string) (string, map[string]string, error)
	Cleanup(ctx context.Context, label string) error
}
