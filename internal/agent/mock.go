package agent

import (
	"context"
	"fmt"
	"regexp"
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

	// Heuristics for specific scenarios (especially E2E smoke tests)

	// 1. TPM Agent (Jira Ticket Generation)
	if strings.Contains(prompt, "You are an expert Technical Program Manager (TPM)") {
		// Extract repo URL if possible (optional, but good for realism)
		repoRegex := regexp.MustCompile(`Repo: (https?://\S+)`)
		repoURL := "https://github.com/example/repo"
		if matches := repoRegex.FindStringSubmatch(prompt); len(matches) > 1 {
			repoURL = matches[1]
		}

		return fmt.Sprintf(`[
  {
    "title": "ID:[PRIMES] Setup Prime Number Generator Project",
    "description": "Initialize the project structure for the prime number generator. Repo: %s",
    "type": "Epic",
    "children": [
      {
        "title": "Implement Sieve of Eratosthenes",
        "description": "Create a Python script that implements the Sieve of Eratosthenes to find prime numbers up to 10000. The script should output the count of primes found. Repo: %s",
        "type": "Story",
        "acceptance_criteria": [
          "Script runs without errors",
          "Outputs correct count of primes",
          "Performance is O(n log log n)"
        ],
        "blocked_by": []
      }
    ]
  }
]`, repoURL, repoURL), nil
	}

	// Return a generic mock response for other cases
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
