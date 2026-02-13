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

	lowerPrompt := strings.ToLower(prompt)

	// Heuristic for 'prime-python' Planning Phase (TPM Agent)
	// Check for "Technical Program Manager" or "generate ticket" which indicates the planning step.
	if strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "generate ticket") {
		return `[
  {
    "title": "ID:[PRIMES] Prime Number Calculation Script",
    "description": "Implement a script to calculate prime numbers. Repo: https://github.com/example/repo",
    "type": "Epic",
    "children": [
      {
        "title": "Implement primes.py",
        "description": "Create a python script that calculates primes < 10,000. Repo: https://github.com/example/repo",
        "type": "Story",
        "acceptance_criteria": [
          "Script creates primes.json",
          "Contains correct prime numbers"
        ],
        "blocked_by": []
      }
    ]
  }
]`, nil
	}

	// Heuristic for 'prime-python' Execution Phase
	if strings.Contains(lowerPrompt, "prime numbers") || strings.Contains(lowerPrompt, "primes.py") {
		return `I will create a Python script to calculate prime numbers.

` + "```bash" + `
cat <<EOF > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [i for i in range(10000) if is_prime(i)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)

print(f"Calculated {len(primes)} prime numbers.")
EOF

python3 primes.py
` + "```" + `

This script calculates primes up to 10,000 and saves them to primes.json.
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
