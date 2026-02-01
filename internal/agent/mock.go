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

	// Heuristic to detect Ticket Generation prompt (e.g. from recac jira generate-from-spec)
	// The prompt usually contains "Technical Program Manager" or "generate-from-spec" context.
	// Since we can't see the exact command context here easily without passing it down,
	// we rely on the prompt content.
	// The CLI error "invalid character 'M' looking for beginning of value" confirms that
	// the CLI expects JSON but got "Mock agent response...".
	if isTicketGenerationPrompt(prompt) {
		return getMockTicketPlan(), nil
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

func isTicketGenerationPrompt(prompt string) bool {
	lower := strings.ToLower(prompt)
	// The prompt template for TPMAgent usually contains these
	return strings.Contains(lower, "technical program manager") ||
		strings.Contains(lower, "create a jira ticket plan") ||
		strings.Contains(lower, "generate-from-spec")
}

func getMockTicketPlan() string {
	// Returns a valid JSON structure matching the ticketNode struct in cmd/recac/jira.go
	return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Implement a CLI tool that generates prime numbers. Repo: https://github.com/example/repo",
    "type": "Epic",
    "children": [
      {
        "title": "ID:[PRIMES-1] Basic Generator Logic",
        "description": "Implement the Sieve of Eratosthenes algorithm.",
        "type": "Story",
        "acceptance_criteria": ["Correctly generates primes up to N"]
      },
      {
        "title": "ID:[PRIMES-2] CLI Interface",
        "description": "Create a CLI command to accept N as input.",
        "type": "Story",
        "acceptance_criteria": ["Accepts --n flag"]
      }
    ]
  }
]`
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
