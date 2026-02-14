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

	// Heuristics for E2E scenarios

	// 1. TPM / Planning Agent (Triggered by "Technical Program Manager")
	// Used in 'recac jira generate-from-spec'
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "agile software development") {
		// Default repo URL if not found in prompt
		repoURL := "https://github.com/process-failed-successfully/recac-jira-e2e"
		// Try to extract Repo from prompt
		if idx := strings.Index(prompt, "Repo: http"); idx != -1 {
			remaining := prompt[idx+6:]
			if endIdx := strings.IndexAny(remaining, "\n\r"); endIdx != -1 {
				repoURL = strings.TrimSpace(remaining[:endIdx])
			} else {
				repoURL = strings.TrimSpace(remaining)
			}
		}

		// Return a valid JSON response with ticket plan
		return fmt.Sprintf(`[
  {
    "title": "ID:[PRIMES] Prime Number Script",
    "description": "Implement a Python script to find prime numbers.\nRepo: %s",
    "type": "Task",
    "children": []
  }
]`, repoURL), nil
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
