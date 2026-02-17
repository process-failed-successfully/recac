package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a simple mock agent for testing and mock mode
// It returns predefined responses without making actual API calls
type MockAgent struct {
	responsePrefix string
	forcedResponse string
}

// NewMockAgent creates a new mock agent
func NewMockAgent() *MockAgent {
	return &MockAgent{
		responsePrefix: "Mock agent response",
	}
}

// SetResponse forces a specific response from the agent
func (m *MockAgent) SetResponse(response string) {
	m.forcedResponse = response
}

// Send implements the Agent interface
// It returns a mock response based on the prompt content to simulate agent behavior in E2E tests
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// 1. Manager/QA Approvals
	if strings.Contains(prompt, "QA Agent") {
		return "QA_PASSED", nil
	}
	if strings.Contains(prompt, "Manager Agent") {
		return "PROJECT_SIGNED_OFF", nil
	}

	// 2. Planning Phase
	// Triggered by "Technical Program Manager" or finding the issue ID "[PRIMES]"
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "ID:[PRIMES]") {
		// Extract Repo URL if possible (simple heuristic)
		repoURL := "https://github.com/example/repo"
		if strings.Contains(prompt, "https://") {
			parts := strings.Split(prompt, " ")
			for _, p := range parts {
				if strings.HasPrefix(p, "https://") {
					repoURL = strings.TrimSpace(p)
					break
				}
			}
		}

		// Return a JSON plan
		return fmt.Sprintf(`[
  {
    "id": "req-1",
    "title": "Implement Prime Number Script",
    "description": "Create a script calculate_primes.py that prints prime numbers. Repo: %s",
    "status": "todo",
    "type": "task"
  }
]`, repoURL), nil
	}

	// 3. Coding Phase
	// Triggered by "Implement" or "write"
	if strings.Contains(prompt, "Implement") || strings.Contains(prompt, "write") {
		return `
cat <<EOF > calculate_primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

for i in range(1, 100):
    if is_prime(i):
        print(i)
EOF
python3 calculate_primes.py
`, nil
	}

	// 4. Task Completion Detection
	// If the prompt contains the output of the script (success), mark as done
	if strings.Contains(prompt, "2\n3\n5\n7") || strings.Contains(prompt, "Recac Finished") {
		return "Task completed", nil
	}

	// Default fallback
	return fmt.Sprintf("%s: I received your prompt (%d chars).", m.responsePrefix, len(prompt)), nil
}

// SendStream implements the Agent interface
func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := m.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(resp)
	}
	return resp, err
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
