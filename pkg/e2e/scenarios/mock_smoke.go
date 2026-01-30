package scenarios

import (
	"fmt"
)

// MockSmokeScenario is a lightweight scenario for CI smoke tests
type MockSmokeScenario struct{}

func (s *MockSmokeScenario) Name() string {
	return "mock-smoke"
}

func (s *MockSmokeScenario) Description() string {
	return "Minimal smoke test using mock provider"
}

func (s *MockSmokeScenario) Generate(uniqueID string, repoURL string) []TicketSpec {
	return []TicketSpec{
		{
			ID:      "PRIMES",
			Summary: fmt.Sprintf("[SMOKE] Hello World [%s]", uniqueID),
			Desc:    fmt.Sprintf("Implement a python script.\nID:[SMOKE-1]\nLabel:%s", uniqueID),
			Type:    "Task",
		},
	}
}

func (s *MockSmokeScenario) AppSpec(repoURL string) string {
	return "Create a python script that prints 'Hello Smoke'"
}

func (s *MockSmokeScenario) Verify(repoPath string, ticketKeys map[string]string) error {
	// No-op verification for smoke test
	return nil
}

func init() {
	Register(&MockSmokeScenario{})
}
