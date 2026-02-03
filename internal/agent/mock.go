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

	// Heuristic: Check for "TPM" or "Technical Program Manager" role to generate a ticket plan
	if strings.Contains(prompt, "TPM") || strings.Contains(prompt, "Technical Program Manager") {
		// Return a valid JSON response for ticket generation (Array of tickets)
		return `[
    {
      "title": "Implement Core Feature",
      "description": "Implement the core functionality as requested.",
      "type": "Epic",
      "children": [
        {
          "title": "Setup Project Structure",
          "description": "Initialize the project structure.",
          "type": "Story"
        },
        {
          "title": "Implement Logic",
          "description": "Write the business logic.",
          "type": "Story"
        }
      ]
    }
  ]`, nil
	}

	// Heuristic: Check if this is the Initializer agent
	if strings.Contains(prompt, "Initializer") || strings.Contains(prompt, "feature_list.json") {
		return "Mock Initializer: Creating feature list.\n```bash\necho '[]' > feature_list.json\n```", nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...\n\n```bash\necho 'Mock Agent: Processing request...'\n```",
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
