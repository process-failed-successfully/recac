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

	// Heuristic: Check if this is a ticket generation request from E2E tests
	// The prompt usually contains the spec content or "ticket"
	if strings.Contains(prompt, "app_spec") || strings.Contains(prompt, "ticket") || strings.Contains(prompt, "Decompose the following application specification") {
		return `[
  {
    "title": "ID:[PRIMES] Prime Number Generator",
    "description": "Implement a prime number generator application based on the specification.\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Epic",
    "children": [
      {
        "title": "ID:[PRIMES-IMP] Implement Prime Check Logic",
        "description": "Implement the core logic to check if a number is prime.\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
        "type": "Story"
      },
      {
        "title": "ID:[PRIMES-CLI] Implement CLI Interface",
        "description": "Implement a CLI to accept user input and display prime numbers.\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
        "type": "Story",
        "blocked_by": ["ID:[PRIMES-IMP]"]
      }
    ]
  }
]`, nil
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
