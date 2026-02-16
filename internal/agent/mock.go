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

	// Heuristics for Smoke Test Scenarios (E2E)

	// 1. Planning Phase (Technical Program Manager)
	// The prompt typically starts with "You are an expert Technical Program Manager (TPM)"
	// or asks for a ticket plan.
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "ticket plan") {
		return `{
  "tickets": [
    {
      "id": "TASK-1",
      "type": "Task",
      "summary": "Implement prime number generator",
      "description": "Create a Python script named primes.py that generates prime numbers up to 100.",
      "status": "Open",
      "priority": "High",
      "assignee": "Unassigned"
    },
    {
      "id": "TASK-2",
      "type": "Task",
      "summary": "Add unit tests for prime generator",
      "description": "Create a test file test_primes.py to verify the prime number generator.",
      "status": "Open",
      "priority": "Medium",
      "assignee": "Unassigned"
    }
  ]
}`, nil
	}

	// 2. Coding Task (Implement prime generator)
	// The prompt might ask to "Implement prime number generator" or mention "primes.py"
	if strings.Contains(prompt, "prime") || strings.Contains(prompt, "primes.py") {
		return "Here is the implementation for `primes.py`:\n\n```python\ndef is_prime(n):\n    if n <= 1:\n        return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0:\n            return False\n    return True\n\ndef generate_primes(limit):\n    primes = []\n    for i in range(2, limit + 1):\n        if is_prime(i):\n            primes.append(i)\n    return primes\n\nif __name__ == '__main__':\n    print(generate_primes(100))\n```\n", nil
	}

	// Return a generic mock response for other cases
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
	return s[:maxLen] + "..."
}
