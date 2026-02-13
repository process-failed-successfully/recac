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

	// Heuristic: Check if this is a Planning / Ticket Generation request
	// The prompt typically starts with "You are an expert Technical Program Manager (TPM)..."
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") {
		// Return a JSON list of tickets for the mock scenario
		// This matches the schema expected by `ticketNode` in `cmd/recac/jira.go`
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Create a Python script that generates prime numbers.",
    "type": "Story",
    "acceptance_criteria": [
      "Must be written in Python",
      "Must handle input n to generate first n primes"
    ],
    "children": [
      {
        "title": "ID:[PRIMES-1] Create primes.py",
        "description": "Repo: https://github.com/example/repo\nImplement the basic prime checking logic.",
        "type": "Task"
      },
      {
        "title": "ID:[PRIMES-2] Add CLI interface",
        "description": "Repo: https://github.com/example/repo\nAdd argparse to handle user input.",
        "type": "Task"
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
