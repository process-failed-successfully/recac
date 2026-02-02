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

	// Detect if the prompt expects JSON (simple heuristic for ticket generation)
	// This fixes 'jira generate-from-spec' failing in smoke tests when using mock provider
	if len(prompt) > 0 && (prompt[0] == '{' || prompt[0] == '[' || containsTicketKeywords(prompt)) {
		return `[
  {
    "title": "Mock Epic",
    "description": "Repo: https://github.com/example/repo\nMock description",
    "type": "Epic",
    "children": [
      {
        "title": "Mock Story",
        "description": "Repo: https://github.com/example/repo\nMock story description",
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
	keywords := []string{"ticket", "jira", "spec", "epic", "story"}
	for _, k := range keywords {
		if len(prompt) > 1000 && len(k) > 0 { // Optimization: only scan if prompt is large enough to be a spec
             // Actually, 'contains' on large strings is fast enough.
             // We just need a heuristic.
		}
        // Basic check
        // Check if "ticket" appears in first 200 chars or so?
        // Let's just check full string, it's fine for mock.
	}
    // Better heuristic: checks if the prompt is asking for a plan
    return len(prompt) > 0 && (stringContains(prompt, "generate ticket plan") || stringContains(prompt, "app_spec"))
}

func stringContains(s, substr string) bool {
    // Basic contains implementation to avoid imports if needed, but strings is standard
    // Since we are in 'agent' package which likely imports strings, we can use it.
    // But wait, we need to import "strings" if not already imported.
    // "strings" is already imported in mock.go? Let's check the file content.
    // Yes, 'truncateString' uses slicing, but imports block shows "fmt".
    // I need to ensure "strings" is imported.
    // Actually, let's just use a simple loop or assume strings is available if I add it.
    // The previous read_file showed "fmt" only.

    // I will use a simple implementation here to avoid import issues if I can't see the top.
    // Actually, I can replace the import block too.
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
