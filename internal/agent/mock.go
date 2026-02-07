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

	// Heuristics for E2E tests

	// 1. Technical Program Manager (Planning) - Generate Tickets
	if strings.Contains(prompt, "Technical Program Manager") && (strings.Contains(prompt, "ticket") || strings.Contains(prompt, "plan")) {
		return `[
  {
    "summary": "[PRIMES] Create Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.",
    "type": "Task",
    "priority": "High",
    "story_points": 5,
    "features": ["feat-core-1"]
  }
]`, nil
	}

	// 2. Lead Software Architect (Architecting) - Generate Feature List
	if strings.Contains(prompt, "Lead Software Architect") && strings.Contains(prompt, "feature") {
		return `{
  "features": [
    {
      "id": "feat-core-1",
      "title": "Core Functionality",
      "description": "The main core logic of the application.",
      "dependencies": []
    },
    {
      "id": "feat-infra-1",
      "title": "Infrastructure Setup",
      "description": "Project setup and configuration.",
      "dependencies": []
    }
  ]
}`, nil
	}

	// 3. Coding Agent (Implementation)
	if strings.Contains(prompt, "Coding Agent") || strings.Contains(prompt, "Developer") {
		return `I have implemented the requested changes.

filename: main.py
code: |
  def main():
      print("Hello from Mock Agent!")

  if __name__ == "__main__":
      main()
`, nil
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
