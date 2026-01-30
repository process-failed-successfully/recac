package scenarios

import (
	"fmt"
)

// MockSmokeScenario is a trivial scenario for checking infrastructure health (Mock Mode).
type MockSmokeScenario struct{}

func (s *MockSmokeScenario) Name() string {
	return "mock-smoke"
}

func (s *MockSmokeScenario) Description() string {
	return "A trivial scenario for verifying CI pipeline infrastructure using Mock Agent."
}

func (s *MockSmokeScenario) AppSpec(repoURL string) string {
	return fmt.Sprintf(`### ID:[MOCK-1] Mock Smoke Test

CRITICAL INSTRUCTION: You MUST create exactly ONE ticket. Type: Task.
Do NOT create an Epic. Do NOT create subtasks.
The ID [MOCK-1] must map to this single Task.

This is a mock task to verify the CI pipeline.
No code needs to be written. The Mock Agent will simulate success.

Repo: %s`, repoURL)
}

func (s *MockSmokeScenario) Generate(uniqueID string, repoURL string) []TicketSpec {
	return []TicketSpec{
		{
			ID:      "MOCK-1",
			Summary: fmt.Sprintf("[%s] Mock Smoke Task", uniqueID),
			Desc:    fmt.Sprintf("This is a mock task for CI smoke testing. Repo: %s", repoURL),
			Type:    "Task",
		},
	}
}

func (s *MockSmokeScenario) Verify(repoPath string, ticketKeys map[string]string) error {
	fmt.Println("Mock Smoke Test Verification: PASSED")
	return nil
}

func init() {
	Register(&MockSmokeScenario{})
}
