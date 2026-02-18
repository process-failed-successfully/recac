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

	// Heuristic: Check if this is a TPM planning prompt (asking for JSON tickets)
	lowerPrompt := strings.ToLower(prompt)
	if strings.Contains(lowerPrompt, "json") && (strings.Contains(lowerPrompt, "ticket") || strings.Contains(lowerPrompt, "epic")) {
		// Return a mock plan for the prime-python scenario or generic
		return m.generateMockPlan(), nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func (m *MockAgent) generateMockPlan() string {
	// Structure matching cmd/recac/jira.go ticketNode
	tickets := []map[string]interface{}{
		{
			"title":              "PRIMES",
			"description":        "Implement a Prime Number Generator in Python. Repo: https://github.com/example/repo",
			"type":               "Epic",
			"blocked_by":         []string{},
			"acceptance_criteria": []string{"Must run correctly"},
			"children": []map[string]interface{}{
				{
					"title":              "Implement generator function",
					"description":        "Write a function to generate primes. Repo: https://github.com/example/repo",
					"type":               "Story",
					"blocked_by":         []string{},
					"acceptance_criteria": []string{"Function exists"},
					"children":           []interface{}{},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(tickets, "", "  ")
	return fmt.Sprintf("Here is the plan:\n```json\n%s\n```", string(data))
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
