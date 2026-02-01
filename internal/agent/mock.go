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
	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys

	// Check if this is a ticket generation request (TPM role)
	// The recac CLI expects a JSON array of tickets in response
	if isTicketGenerationPrompt(prompt) {
		return `[{"id": "MOCK-1", "title": "Implement Core Features", "description": "Implement the core functionality as requested in the specification. Repo: https://github.com/example/repo", "type": "Task"}]`, nil
	}

	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

// isTicketGenerationPrompt checks if the prompt is asking for ticket generation
func isTicketGenerationPrompt(prompt string) bool {
	// Look for keywords associated with the TPM prompt in cmd/recac/generate.go
	return len(prompt) > 0 && (contains(prompt, "Technical Program Manager") || contains(prompt, "generate ticket") || contains(prompt, "ticket plan"))
}

func contains(s, substr string) bool {
	// Simple containment check, can be replaced with strings.Contains
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
