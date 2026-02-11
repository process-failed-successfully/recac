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

	// Heuristic for TPM Agent (Jira Planning)
	// The E2E test uses a specific prompt header for the TPM role
	if strings.Contains(prompt, "You are an expert Technical Program Manager (TPM)") {
		return m.generateMockTPMResponse(), nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

// generateMockTPMResponse returns a JSON structure matching the ticketNode struct expected by recac jira
func (m *MockAgent) generateMockTPMResponse() string {
	// Define structs locally to avoid circular dependencies with main package
	type ticketNode struct {
		Title              string       `json:"title"`
		Description        string       `json:"description"`
		Type               string       `json:"type"`
		BlockedBy          []string     `json:"blocked_by"`
		AcceptanceCriteria []string     `json:"acceptance_criteria"`
		Children           []ticketNode `json:"children"`
	}

	// Create a simple plan for the prime number script
	plan := []ticketNode{
		{
			Title:       "ID:[PRIMES] Implement Prime Number Script",
			Description: "Create a Python script to calculate prime numbers.",
			Type:        "Epic",
			Children: []ticketNode{
				{
					Title:              "ID:[PRIME-1] Create primes.py",
					Description:        "Implement the main script logic.",
					Type:               "Story",
					AcceptanceCriteria: []string{"Script runs without errors", "Calculates primes correctly"},
				},
			},
		},
	}

	data, _ := json.Marshal(plan)
	return string(data)
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
