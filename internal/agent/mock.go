package agent

import (
	"context"
	"fmt"
	"os"
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

	lowerPrompt := strings.ToLower(prompt)

	// Heuristic for TPM / Ticket Generation
	// If the prompt asks for a ticket plan or identifies as a TPM, return a valid JSON list of tickets
	if strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "generate a ticket plan") {
		return `[
  {
    "id": "TASK-1",
    "title": "Implement Prime Number Generator",
    "description": "Create a Python script that generates prime numbers.",
    "type": "Task",
    "status": "Open",
    "dependencies": []
  },
  {
    "id": "TASK-2",
    "title": "Write Unit Tests for Primes",
    "description": "Write unit tests to verify the prime number generator.",
    "type": "Task",
    "status": "Open",
    "dependencies": ["TASK-1"]
  }
]`, nil
	}

	// Heuristic for Primes coding task (Agent execution)
	if strings.Contains(lowerPrompt, "prime") || strings.Contains(lowerPrompt, "primes.json") || strings.Contains(os.Getenv("RECAC_INJECTED_FEATURES"), "prime") {

		// 1. Detect Successful Execution (Prompt contains output like JSON list or "OK")
		// This indicates the task is done, so we mark the feature as complete.
		// We append "|| true" to prevent failure if the feature doesn't exist in the DB (common in smoke tests).
		if strings.Contains(lowerPrompt, "[2, 3, 5") || strings.Contains(lowerPrompt, "ok") || strings.Contains(lowerPrompt, "ran 2 tests") {
			return `I see the tests passed or the output is correct. I will mark the task as complete.

` + "```bash" + `
agent-bridge feature set $RECAC_PROJECT_ID --status done --passes true || true
` + "```" + `
`, nil
		}

		// 2. Detect "Write Unit Tests" Request
		if strings.Contains(lowerPrompt, "test") {
			return `I will write the unit tests for the prime number generator.

` + "```bash" + `
cat <<EOF > test_primes.py
import unittest
import json
from primes import is_prime, generate_primes

class TestPrimes(unittest.TestCase):
    def test_is_prime(self):
        self.assertFalse(is_prime(1))
        self.assertTrue(is_prime(2))
        self.assertTrue(is_prime(3))
        self.assertFalse(is_prime(4))
        self.assertTrue(is_prime(5))

    def test_generate_primes(self):
        self.assertEqual(generate_primes(5), [2, 3, 5, 7, 11])

if __name__ == '__main__':
    unittest.main()
EOF

python3 test_primes.py
` + "```" + `
`, nil
		}

		// 3. Detect "Implement/Create" Request (Implementation)
		if strings.Contains(lowerPrompt, "implement") || strings.Contains(lowerPrompt, "create") || strings.Contains(lowerPrompt, "write") {
			return `I will create the prime number generator script.

` + "```bash" + `
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

def generate_primes(n):
    primes = []
    num = 2
    while len(primes) < n:
        if is_prime(num):
            primes.append(num)
        num += 1
    return primes

if __name__ == "__main__":
    import json
    print(json.dumps(generate_primes(10)))
EOF

python3 primes.py
` + "```" + `
`, nil
		}
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
