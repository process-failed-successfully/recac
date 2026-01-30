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

	// Check if this is a ticket generation request (approximate check)
	if isTicketGenerationRequest(prompt) {
		return mockTicketResponse(), nil
	}

	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func isTicketGenerationRequest(prompt string) bool {
	// Check for keywords used in the TPM agent prompt or spec file
	// See cmd/recac/jira.go and internal/agent/prompts/templates/tpm_agent.md
	return len(prompt) > 0 && (
		(contains(prompt, "Technical Program Manager") && contains(prompt, "Jira")) ||
		contains(prompt, "app_spec.txt") ||
		contains(prompt, "ID:[PRIMES]")) // Specific to the e2e test scenario
}

func contains(s, substr string) bool {
	// Simple wrapper for strings.Contains to avoid importing strings if not already imported
	// But we need to import strings for this to be robust or just implement search.
	// Actually, let's just use strings.Contains and ensure import.
	// But wait, I need to check imports.
    // Assuming standard library usage.
    for i := 0; i < len(s)-len(substr)+1; i++ {
        if s[i:i+len(substr)] == substr {
            return true
        }
    }
    return false
}

func mockTicketResponse() string {
	return `{
  "epics": [
    {
      "title": "ID:[PRIMES] Prime Number Service",
      "description": "Implementation of a service to calculate prime numbers.",
      "type": "Epic",
      "children": [
        {
          "title": "ID:[PRIMES-CLI] Implement CLI",
          "description": "Create a CLI tool to accept a number and return primes up to that number.",
          "type": "Story",
          "acceptance_criteria": [
            "CLI accepts --n flag",
            "CLI prints primes to stdout"
          ]
        }
      ]
    }
  ]
}`
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
