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

	// Heuristics for specific roles/scenarios
	// 1. Technical Program Manager (Jira Generation)
	if contains(prompt, "Technical Program Manager") {
		return `[
  {
    "title": "ID:[EPIC-1] Implement Core Features",
    "description": "Implement the core features of the system.\n\nRepo: https://github.com/example/repo",
    "type": "Epic",
    "children": [
      {
        "title": "ID:[STORY-1] Setup Project Structure",
        "description": "Initialize the repository and project structure.\n\nRepo: https://github.com/example/repo",
        "type": "Story",
        "acceptance_criteria": ["Git repo initialized", "Go module created"]
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
	return len(s) >= len(substr) && (s == substr || len(substr) > 0 &&
		(s[0:len(substr)] == substr || len(s) > len(substr) &&
		search(s, substr)))
}

func search(s, substr string) bool {
	// Simple substring check to avoid "strings" import if strictly needed,
	// but standard library is allowed. Let's strictly use "strings" in imports if possible.
	// But to avoid adding import in diff block blindly, let's assume we can add it or write a helper.
	// Since "fmt" and "context" are there, let's just stick to "strings" if I can ensure it's imported.
	// Wait, I can't easily add import with replace block unless I target the top.
	// I'll implement a simple naive loop or just trust I can Replace the top imports too.
	// Actually, I'll update the file content properly.
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
