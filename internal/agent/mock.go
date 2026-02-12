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

	// Heuristics for different agent roles/scenarios
	lowerPrompt := strings.ToLower(prompt)

	// 1. TPM/Architect/Manager Agent (generating tickets)
	// Matches: "TPM", "spec", "agile", "user story", "generate ticket plan"
	if strings.Contains(lowerPrompt, "tpm") || strings.Contains(lowerPrompt, "spec") || strings.Contains(lowerPrompt, "user story") {
		return `[
  {
    "title": "ID:[PRIMES] Implement Primes Service",
    "description": "Implement a service to calculate prime numbers. Repo: https://github.com/example/repo",
    "type": "Epic",
    "children": [
      {
        "title": "ID:[PRIMES-1] Basic Prime Calculation",
        "description": "Implement the basic algorithm.",
        "type": "Story",
        "acceptance_criteria": [
          "Returns true for 2, 3, 5, 7",
          "Returns false for 1, 4, 6, 8, 9"
        ]
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
