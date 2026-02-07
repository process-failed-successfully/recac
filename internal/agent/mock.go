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

	// Heuristics for Smoke Test Scenarios

	// 1. TPM Agent (Planning)
	// The CLI expects a raw JSON array for the TPM role.
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "Ticket Generation") {
		return `[
  {
    "id": "PRIMES",
    "name": "Implement Prime Number Checker",
    "type": "Task",
    "description": "Create a python script that checks for prime numbers.",
    "dependencies": []
  }
]`, nil
	}

	// 2. Developer Agent (Implementation)
	// The agent needs to implement the requested task.
	// For the prime-python scenario, it typically asks to implement the prime checker.
	if strings.Contains(prompt, "Developer") || strings.Contains(prompt, "Implement") || strings.Contains(prompt, "PRIMES") || strings.Contains(prompt, "prime number") {
		return `Plan: Implement the prime checker in Python.
Command: cat <<EOF > primes.py
import sys
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

if __name__ == "__main__":
    if len(sys.argv) > 1:
        n = int(sys.argv[1])
        print(is_prime(n))
    else:
        # Generate primes.json for verification
        data = {"primes": [p for p in range(100) if is_prime(p)]}
        with open("primes.json", "w") as f:
            json.dump(data, f)
EOF
Command: python3 primes.py
Command: git add primes.py primes.json
Command: git commit -m "Implement prime checker"
Command: agent-bridge feature set --id PRIMES --status done --passes true
`, nil
	}

	// 3. Initializer (Feature List)
	if strings.Contains(prompt, "INITIALIZER") || strings.Contains(prompt, "feature list") {
		return `Plan: Initialize project features.
Command: cat <<EOF | agent-bridge import
{
  "project_name": "smoke-test",
  "features": [
    {
      "id": "req-must-correctly-identify-prime-",
      "description": "Must correctly identify prime numbers",
      "category": "functional",
      "priority": "high",
      "status": "pending"
    }
  ]
}
EOF`, nil
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
