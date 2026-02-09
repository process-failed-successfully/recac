package agent

import (
	"context"
	"encoding/json"
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

	// TPM (Technical Program Manager) - Ticket Generation
	if strings.Contains(prompt, "Technical Program Manager") {
		tickets := []map[string]interface{}{
			{
				"title":       "Implement Primes",
				"description": "Create a python script that prints prime numbers up to 100",
				"type":        "Task",
			},
		}
		data, _ := json.Marshal(tickets)
		return string(data), nil
	}

	// Initializer Agent - Feature List
	if strings.Contains(prompt, "CREATE FEATURE_LIST.JSON") {
		features := map[string]interface{}{
			"features": []string{"prime-numbers"},
		}
		data, _ := json.Marshal(features)
		return fmt.Sprintf("```json\n%s\n```", string(data)), nil
	}

	// Coding Agent - Implementation
	if strings.Contains(prompt, "prime numbers") || strings.Contains(prompt, "Implement Primes") {
		return "Here is the python script:\n```python\nprint('2, 3, 5, 7, ...')\n```", nil
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
