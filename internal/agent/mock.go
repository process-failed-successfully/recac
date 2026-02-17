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
// It returns a mock response that acknowledges the prompt
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// 1. TPM / Planning Phase Detection
	// The prompt usually starts with "You are an expert Technical Program Manager" or similar context
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "app_spec.txt") {
		// Return a valid JSON ticket plan for the Primes scenario
		return `[
  {
    "title": "ID:[PRIMES] Implement prime number generator",
    "description": "Implement a Python script that generates prime numbers up to 10000. It should output the result as a JSON object to stdout.",
    "type": "Story",
    "acceptance_criteria": [
      "Generates primes up to 10000",
      "Output is valid JSON",
      "Includes unit tests"
    ]
  }
]`, nil
	}

	// 2. Coding Phase Detection
	// The prompt asks to implement the ticket ID:[PRIMES]
	if strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "prime number generator") {
		// Return a Bash script that implements the solution and tests
		return `Here is the implementation:

` + "```bash" + `
cat <<EOF > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(10000) if is_prime(x)]
print(json.dumps({"primes": primes}))
EOF

cat <<EOF > test_primes.py
import unittest
from primes import is_prime

class TestPrimes(unittest.TestCase):
    def test_primes(self):
        self.assertTrue(is_prime(7))
        self.assertFalse(is_prime(4))

if __name__ == '__main__':
    unittest.main()
EOF

# Run tests
python3 test_primes.py

# Run main script to satisfy verification
python3 primes.py
` + "```" + `
`, nil
	}

	// Default / Fallback Response
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
