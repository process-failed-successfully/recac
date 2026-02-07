package agent

import (
	"context"
	"fmt"
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

	// Heuristic: Check if this is a TPM Agent prompt expecting JSON
	// The TPM Agent prompt typically contains "Technical Program Manager" and "Application Specification"
	if len(prompt) > 0 && (contains(prompt, "Technical Program Manager") || contains(prompt, "Application Specification")) {
		// Return a valid JSON response matching the expected ticketNode structure
		return `[
  {
    "title": "Mock Epic",
    "description": "A high-level overview of the mock feature area.\n\nRepo: https://github.com/example/repo",
    "type": "Epic",
    "children": [
      {
        "title": "Mock Story",
        "description": "A concise description of the mock task.\n\nRepo: https://github.com/example/repo",
        "type": "Story",
        "acceptance_criteria": [
          "Mock criteria 1",
          "Mock criteria 2"
        ],
        "blocked_by": []
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
	return len(s) >= len(substr) && len(substr) > 0 && search(s, substr)
}

func search(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
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
