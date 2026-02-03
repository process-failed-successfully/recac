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

	// Detect if the prompt expects JSON (simple heuristic for ticket generation)
	// This fixes 'jira generate-from-spec' failing in smoke tests when using mock provider
	if len(prompt) > 0 && (prompt[0] == '{' || prompt[0] == '[' || containsTicketKeywords(prompt)) {
		return `[
  {
    "title": "Mock Epic",
    "description": "Repo: https://github.com/process-failed-successfully/recac-jira-e2e\nMock description",
    "type": "Epic",
    "children": [
      {
        "title": "Mock Story",
        "description": "Repo: https://github.com/process-failed-successfully/recac-jira-e2e\nMock story description",
        "type": "Story"
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

func containsTicketKeywords(prompt string) bool {
	// Simple check for keywords likely present in ticket generation prompts
	keywords := []string{"ticket", "jira", "spec", "epic", "story", "Technical Program Manager", "application specification"}
	for _, k := range keywords {
		if strings.Contains(prompt, k) {
			return true
		}
	}
	// Better heuristic: checks if the prompt is asking for a plan
	return len(prompt) > 0 && (strings.Contains(prompt, "generate ticket plan") || strings.Contains(prompt, "app_spec"))
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
