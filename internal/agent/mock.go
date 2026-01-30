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

	// Mock Logic for E2E Tests
	// 1. Ticket Generation (Jira) - Checks for project ID or "ticket" keywords if typically used
	// The E2E test uses "ID:[PRIMES]" in the spec to map tickets.
	// We need to return a JSON list of tickets.
	// Based on the error: "failed to parse agent response as JSON"
	// We should return valid JSON.
	if len(prompt) > 0 {
		// Heuristic: If it looks like a Jira plan request (e.g. contains "User Story" or "Task")
		// For the 'prime-python' scenario, it likely asks for a plan to implement primes.
		// Let's return a generic valid ticket plan.
		// We can detect "ID:[...]" from the prompt or just always return JSON if it looks like a plan request.
		// However, simple text prompts might also come through.
		// Let's try to be specific to the known E2E failure.
		// The prompt preview in logs was: "You are an expert Technical Program Manager..."
		// This suggests the Jira ticket generation step.

		// Check for specific keywords related to the E2E test
		// The prompt usually contains the spec.
		// "prime-python" scenario probably mentions "primes".
		// "http-proxy" mentions "proxy".
		// We'll return a generic plan that satisfies the JSON parser.
		if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "Jira") {
			return `[
  {
    "id": "PRIMES",
    "title": "Implement Prime Number Generator",
    "description": "Create a Python script that generates prime numbers.",
    "type": "Story",
    "acceptance_criteria": [
      "Must correctly identify primes",
      "Must be efficient"
    ],
    "dependencies": []
  }
]`, nil
		}

		// 2. Code Generation (Agent) - Checks for coding keywords
		// If the prompt asks for code (e.g. "Implement this..."), return code.
		// The E2E runner expects the agent to push code.
		// For 'prime-python', we need a python script.
		if strings.Contains(prompt, "python") && strings.Contains(prompt, "prime") {
			return `Here is the python code:
` + "```python" + `
def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

if __name__ == "__main__":
    import sys
    # Simple test
    print([x for x in range(20) if is_prime(x)])
` + "```", nil
		}
	}

	// Default fallback
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
