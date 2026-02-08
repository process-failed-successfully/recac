package agent

import (
	"context"
	"encoding/json"
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
// It returns a mock response that acknowledges the prompt
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristic 1: TPM / Ticket Generation for Smoke Test
	if strings.Contains(prompt, "Technical Program Manager") {
		type ticketNode struct {
			Title              string       `json:"title"`
			Description        string       `json:"description"`
			Type               string       `json:"type"`
			BlockedBy          []string     `json:"blocked_by"`
			AcceptanceCriteria []string     `json:"acceptance_criteria"`
			Children           []ticketNode `json:"children"`
		}

		tickets := []ticketNode{
			{
				Title:       "ID:[PRIMES] Implement Prime Number Service",
				Description: "Implement a python service to check for prime numbers.\n\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
				Type:        "Task",
				Children:    []ticketNode{},
			},
		}
		data, err := json.Marshal(tickets)
		if err == nil {
			return string(data), nil
		}
	}

	// Heuristic 2: Coding Phase for Smoke Test (Prime Python)
	lowerPrompt := strings.ToLower(prompt)
	if (strings.Contains(lowerPrompt, "implement") || strings.Contains(lowerPrompt, "create")) &&
		(strings.Contains(lowerPrompt, "prime") || strings.Contains(lowerPrompt, "primes")) {

		return `I will implement the prime number checking service.

` + "```bash" + `
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

if __name__ == "__main__":
    import sys
    if len(sys.argv) > 1:
        print(is_prime(int(sys.argv[1])))
EOF

cat <<EOF > test_primes.py
import unittest
from primes import is_prime

class TestPrimes(unittest.TestCase):
    def test_prime(self):
        self.assertTrue(is_prime(7))
        self.assertFalse(is_prime(4))
        self.assertFalse(is_prime(1))

if __name__ == '__main__':
    unittest.main()
EOF

git add primes.py test_primes.py
git commit -m "Implement primes service"
git push
` + "```" + `

I have completed the task.
`, nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
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
