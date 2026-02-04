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
