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

	// Heuristic: Check for Technical Program Manager prompt (Jira ticket generation)
	if strings.Contains(prompt, "Technical Program Manager") {
		return m.generateMockTickets(), nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func (m *MockAgent) generateMockTickets() string {
	return "```json\n" + `[
  {
    "title": "Mock Epic: System Implementation",
    "description": "Repo: https://example.com/repo\nImplement the core system.",
    "type": "Epic",
    "children": [
      {
        "title": "Mock Story: Backend API",
        "description": "Implement the API endpoints.",
        "type": "Story",
        "acceptance_criteria": ["API responds to health check"],
        "children": []
      },
      {
        "title": "Mock Story: Frontend UI",
        "description": "Implement the dashboard.",
        "type": "Story",
        "acceptance_criteria": ["Dashboard loads"],
        "children": []
      }
    ]
  }
]` + "\n```"
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
