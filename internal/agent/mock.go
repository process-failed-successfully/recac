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

	promptLower := strings.ToLower(prompt)

	// Heuristic 1: Technical Program Manager (TPM) - Ticket Generation
	if strings.Contains(promptLower, "technical program manager") || strings.Contains(promptLower, "tpm") {
		return `[
  {
    "title": "ID:[PRIMES] Prime Number Script",
    "description": "Implement a Python script to calculate prime numbers. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Story",
    "acceptance_criteria": [
      "Script calculates primes correctly",
      "Unit tests are included"
    ],
    "children": []
  }
]`, nil
	}

	// Heuristic 2: Coding Agent
	if strings.Contains(promptLower, "coding agent") || strings.Contains(promptLower, "you are an expert software engineer") {
		return "Here is the implementation for the prime number script:\n\n```python\ndef is_prime(n):\n    if n <= 1:\n        return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0:\n            return False\n    return True\n\nif __name__ == '__main__':\n    import sys\n    print(is_prime(int(sys.argv[1])))\n```", nil
	}

	// Heuristic 3: QA Agent
	if strings.Contains(promptLower, "qa agent") || strings.Contains(promptLower, "quality assurance") {
		return "QA_PASSED", nil
	}

	// Heuristic 4: Manager Review
	if strings.Contains(promptLower, "manager review") || strings.Contains(promptLower, "project manager") {
		return "LGTM", nil
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
