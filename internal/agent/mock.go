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

	// Special heuristic for E2E tests:
	// If the prompt is for the TPM (generate-from-spec), return a valid JSON mapping.
	// The prompt usually contains "Technical Program Manager" or asks for JSON output mapping IDs.
	if containsTPMKeywords(prompt) {
		// Return a JSON mapping that satisfies the E2E test expectations.
		// The E2E test usually looks for 'PRIMES' or generic components.
		// We return a mapping that maps typical IDs to a mock ticket key.
		return `
{
  "ID:[PRIMES]": "MFLP-1",
  "ID:[PRIME_GENERATOR]": "MFLP-2",
  "ID:[HTTP_SERVER]": "MFLP-3",
  "ID:[LOAD_BALANCER]": "MFLP-4"
}`, nil
	}

	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func containsTPMKeywords(s string) bool {
	// Simple check for keywords used in the TPM prompt
	return (len(s) > 0 && (contains(s, "Technical Program Manager") || contains(s, "TPM"))) && contains(s, "JSON")
}

func contains(s, substr string) bool {
	// Simple containment check, could use strings.Contains but we don't import strings yet
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
