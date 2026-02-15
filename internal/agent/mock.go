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

	// Heuristic: If the prompt asks for a Technical Program Manager (TPM) or JSON,
	// return a valid JSON structure for ticket generation.
	// This is required for 'recac jira generate-from-spec' to work in smoke tests.
	if (len(prompt) > 0 && (contains(prompt, "Technical Program Manager") || contains(prompt, "TPM"))) && contains(prompt, "json") {
		return `[
  {
    "title": "ID:[SMOKE-EPIC] Smoke Test Epic",
    "description": "This is a generated epic for smoke testing.",
    "type": "Epic",
    "children": [
      {
        "title": "ID:[SMOKE-TASK-1] Implement Prime Number Generator",
        "description": "Implement a Python script that generates prime numbers up to N.\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
        "type": "Story",
        "acceptance_criteria": ["Script runs successfully", "Output is correct"],
        "children": []
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

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
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
