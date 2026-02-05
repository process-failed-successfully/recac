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
func NewMockAgent(model, project string) *MockAgent {
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
	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys

	// Heuristic for Initializer Agent
	if strings.Contains(prompt, "YOUR ROLE - INITIALIZER AGENT") {
		return `{"files": ["primes.py"]}`, nil
	}

	// Heuristic to detect TPM / Planner prompt requesting JSON tickets
	if isTPMPrompt(prompt) {
		return `[
  {
    "title": "ID:[PRIMES] Prime Number Generator System",
    "description": "Implementation of prime number generator system.\n\nRepo: https://github.com/example/repo",
    "type": "Epic",
    "children": [
      {
        "title": "ID:[PRIMES-1] Implement Core Generator",
        "description": "Implement the core logic for generating prime numbers.\n\nRepo: https://github.com/example/repo",
        "type": "Story",
        "acceptance_criteria": [
          "Must correctly identify prime numbers",
          "Must print primes up to 20"
        ]
      }
    ]
  }
]`, nil
	}

	// Heuristic for Coding Agent (PRIMES)
	if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py") {
		return `I will implement the prime number generator.

` + "```bash" + `
#!/bin/bash
set -x

# Create the python script
cat <<EOF > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [x for x in range(10000) if is_prime(x)]
result = {"primes": primes}

with open("primes.json", "w") as f:
    json.dump(result, f)
EOF

# Run it
python3 primes.py

# Signal completion
agent-bridge signal set COMPLETED true
` + "```", nil
	}

	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func isTPMPrompt(prompt string) bool {
	// Check for keywords common in TPM/Planner prompts
	keywords := []string{
		"Technical Program Manager",
		"ticket plan",
		"generate tickets",
		"app_spec",
		"JSON list of tickets",
	}
	promptLower := strings.ToLower(prompt)
	for _, kw := range keywords {
		if strings.Contains(promptLower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
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
