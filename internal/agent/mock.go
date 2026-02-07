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
// This allows the session to run without requiring real API keys
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristics for E2E tests

	// 1. Technical Program Manager (TPM) - Ticket Generation
	if strings.Contains(prompt, "Technical Program Manager (TPM)") && strings.Contains(prompt, "ticket") {
		// Return a valid JSON array of tickets as expected by the recac jira generate-from-spec command
		return `[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Implement a Python script that calculates prime numbers up to 10000. \n\nRepo: https://github.com/example/repo",
    "type": "Story",
    "acceptance_criteria": [
      "Script prints primes to stdout",
      "Script is idempotent"
    ],
    "children": []
  }
]`, nil
	}

	// 2. Developer / Coding Agent - Primes
	if strings.Contains(prompt, "Prime Number Script") || strings.Contains(prompt, "calculate prime numbers") {
		return "```python\nprint(\"Calculating primes...\")\nprimes = [x for x in range(2, 10001) if all(x % i != 0 for i in range(2, int(x**0.5) + 1))]\nimport json\nprint(json.dumps({\"primes\": primes}))\n```", nil
	}

	// Default response
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
