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

	// Heuristic 1: Jira Ticket Generation (TPM)
	// Trigger: "Technical Program Manager" or "ROLE - PROJECT MANAGER"
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "ROLE - PROJECT MANAGER") {
		// Return valid JSON for ticket generation
		// Format: Array of ticket objects
		// IMPORTANT: Title must contain "ID:[PRIMES]" or similar to match regex extraction if needed
		// For smoke tests, we usually want a simple plan.

		// Check if it's the [PRIMES] scenario
		if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "prime number") {
			return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Create a Python script ` + "`primes.py`" + ` that calculates prime numbers up to 100.",
    "type": "Story",
    "acceptance_criteria": [
      "Script runs without errors",
      "Outputs correct prime numbers"
    ],
    "story_points": 3
  },
  {
    "title": "ID:[PRIMES] Add Unit Tests for Primes",
    "description": "Create ` + "`test_primes.py`" + ` to verify the generator.",
    "type": "Story",
    "acceptance_criteria": [
      "Tests cover edge cases",
      "All tests pass"
    ],
    "story_points": 2,
    "dependencies": ["ID:[PRIMES] Implement Prime Number Generator"]
  }
]`, nil
		}
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
