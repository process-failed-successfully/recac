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

	primeCodeResponse := `I will implement the prime number generator in Python.

` + "```bash" + `
cat << 'EOF' > primes.py
import sys

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

def generate_primes(limit):
    primes = []
    for num in range(2, limit + 1):
        if is_prime(num):
            primes.append(num)
    return primes

if __name__ == "__main__":
    limit = 100
    if len(sys.argv) > 1:
        try:
            limit = int(sys.argv[1])
        except ValueError:
            pass

    primes = generate_primes(limit)
    print(primes)
EOF
` + "```" + `

And I'll run it to verify:

` + "```bash" + `
python3 primes.py 50
` + "```"

	// Heuristics for smoke tests
	isPlanning := strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") || strings.Contains(prompt, "ticket generation")
	isPrimeTask := strings.Contains(prompt, "prime") || strings.Contains(prompt, "primes.json")
	isCodingAgent := strings.Contains(prompt, "CODING AGENT")

	// 1. Coding Task (Execution Phase)
	// Prioritize Coding Agent execution to avoid false positives from history context containing TPM keywords
	if (isCodingAgent && isPrimeTask) || (!isPlanning && isPrimeTask) {
		return primeCodeResponse, nil
	}

	// 2. Ticket Generation (Planning Phase)
	if isPlanning {
		return `[
  {
    "summary": "Implement prime number generator",
    "description": "Create a Python script that generates prime numbers up to a specified limit. The script should be efficient and well-documented.",
    "type": "Task",
    "priority": "High",
    "labels": ["backend", "python"],
    "acceptance_criteria": [
      "Script accepts a limit as a command-line argument",
      "Outputs prime numbers to stdout",
      "Includes unit tests"
    ]
  }
]`, nil
	}

	// Default Mock Response
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
