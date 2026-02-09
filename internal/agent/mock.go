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
	// Check for TPM prompt (used in recac jira generate-from-spec)
	if isTPMPrompt(prompt) {
		return `[
  {
    "title": "ID:[PRIMES] Create primes.py",
    "description": "Create a python script that calculates primes up to 10000. Repo: https://github.com/example/repo",
    "type": "Epic",
    "children": [
      {
        "title": "Implement Prime Calculation",
        "description": "Implement the sieve of Eratosthenes. Repo: https://github.com/example/repo",
        "type": "Story",
        "acceptance_criteria": [
          "Script runs without errors",
          "Calculates primes correctly"
        ]
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

func isTPMPrompt(prompt string) bool {
	// Check for key phrases from tpm_agent.md
	return (contains(prompt, "Technical Program Manager") || contains(prompt, "TPM")) &&
		contains(prompt, "decompose it into a series of high-quality")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && len(s) > 0 && len(substr) > 0 && func() bool {
		// Basic substring check loop to avoid importing strings if we want to minimize deps
		// But since we are mocking an AI, let's just use a loop.
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}()
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
