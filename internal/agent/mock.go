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

	// Check if this looks like a TPM Planning prompt (requires JSON)
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "ID:[PRIMES]") {
		// Extract repo URL from prompt if possible (simple heuristic)
		repoURL := "https://github.com/example/repo"
		if strings.Contains(prompt, "Repo: http") {
			parts := strings.SplitAfter(prompt, "Repo: ")
			if len(parts) > 1 {
				repoURL = strings.Split(parts[1], "\n")[0]
			}
		}

		// Return a valid JSON response for the planning agent
		return fmt.Sprintf(`[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Implement a Python script to generate prime numbers.\n\nRepo: %s",
    "type": "Story",
    "acceptance_criteria": [
      "Script accepts N as argument",
      "Prints first N primes"
    ],
    "children": []
  }
]`, repoURL), nil
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
