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

	// Heuristics for Smoke Test Compliance
	// 1. TPM / Jira Ticket Generation
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") {
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "type": "Epic",
    "description": "Implement a Python script to generate prime numbers.",
    "acceptance_criteria": [
      "Must generate primes up to N",
      "Must be efficient"
    ],
    "dependencies": []
  },
  {
    "title": "ID:[PRIMES-1] Create primes.py",
    "type": "Task",
    "description": "Write the core logic for finding primes.",
    "acceptance_criteria": [
      "File primes.py exists",
      "Function generate_primes(n) is implemented"
    ],
    "dependencies": ["ID:[PRIMES]"]
  }
]`, nil
	}

	// 2. Coding / Script Generation (PRIMES)
	if strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "primes.py") {
		return `#!/bin/bash
# Implement primes.py
cat <<EOF > primes.py
def generate_primes(n):
    primes = []
    for i in range(2, n + 1):
        is_prime = True
        for j in range(2, int(i ** 0.5) + 1):
            if i % j == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(i)
    return primes

if __name__ == "__main__":
    import sys
    n = int(sys.argv[1]) if len(sys.argv) > 1 else 10
    print(generate_primes(n))
EOF

# Ensure git configuration
git config user.email "you@example.com"
git config user.name "Your Name"

# Commit and Push
git add primes.py
git commit -m "Implement primes.py"
git push origin HEAD
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
