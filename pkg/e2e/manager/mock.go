package manager

import (
	"context"
	"fmt"
	"log"
)

// MockTicketManager simulates TicketManager for smoke tests
type MockTicketManager struct{}

func NewMockTicketManager() *MockTicketManager {
	return &MockTicketManager{}
}

func (m *MockTicketManager) GenerateScenario(ctx context.Context, scenarioName, repoURL, provider, model string) (string, map[string]string, error) {
	log.Printf("[Mock] Generating scenario %s (provider: %s)", scenarioName, provider)
	label := "recac-e2e-mock"
	ticketMap := map[string]string{
		"PRIMES": "MOCK-1",
	}
	log.Printf("[Mock] Generated label: %s, TicketMap: %v", label, ticketMap)
	return label, ticketMap, nil
}

func (m *MockTicketManager) Cleanup(ctx context.Context, label string) error {
	fmt.Printf("[Mock] Cleanup called for label %s\n", label)
	return nil
}
