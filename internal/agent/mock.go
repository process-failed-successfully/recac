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

	// Heuristics for CI/Smoke Tests to return valid format
	lowerPrompt := strings.ToLower(prompt)

	// 1. TPM Role - Expects JSON Ticket List
	if strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "tpm") {
		return `[
  {
    "id": "TASK-1",
    "title": "Mock Task for CI",
    "description": "This is a generated mock task for the CI smoke test.",
    "type": "Task",
    "status": "To Do",
    "priority": "Medium",
    "dependencies": []
  }
]`, nil
	}

	// 2. QA Role - Expects JSON Report or specific format
	if strings.Contains(lowerPrompt, "qa") && strings.Contains(lowerPrompt, "report") {
		return `{"status": "passed", "details": "Mock QA passed successfully."}`, nil
	}

	// Default: Return a mock response that shows the agent received the prompt
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
