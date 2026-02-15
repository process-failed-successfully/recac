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

	lowerPrompt := strings.ToLower(prompt)

	// Heuristic for Architect/TPM JSON requests
	// The TPM prompt asks for "Output purely JSON" and mentions "Technical Program Manager"
	// The Architect prompt also asks for JSON schemas.
	if (strings.Contains(lowerPrompt, "json") || strings.Contains(lowerPrompt, "schema")) &&
		(strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "architect") || strings.Contains(lowerPrompt, "output purely json")) {

		// Return a valid JSON structure for tickets
		return `[
  {
    "title": "Prime Number Script",
    "description": "Implement a Python script to check for prime numbers.\n\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Epic",
    "children": [
      {
        "title": "ID:[PRIMES] Implement is_prime function",
        "description": "Create a function that returns true if a number is prime.\n\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
        "type": "Story",
        "acceptance_criteria": [
          "Function returns true for 2, 3, 5, 7",
          "Function returns false for 4, 6, 8, 9",
          "Function handles edge cases like 0 and 1"
        ],
        "blocked_by": []
      },
      {
        "title": "Create main execution script",
        "description": "Create a script that uses the function to print primes.\n\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
        "type": "Story",
        "acceptance_criteria": [
          "Script runs without errors",
          "Script prints primes up to 100"
        ],
        "blocked_by": ["Implement is_prime function"]
      }
    ]
  }
]`, nil
	}

	// Heuristic for Initializer/Planner JSON requests
	if strings.Contains(lowerPrompt, "planner") || strings.Contains(lowerPrompt, "feature_list.json") {
		return `{
  "project_name": "Mock Project",
  "features": [
    {
      "id": "feature-1",
      "category": "functional",
      "description": "Implement basic functionality",
      "status": "pending",
      "steps": ["Create file", "Run test"],
      "dependencies": {}
    }
  ]
}`, nil
	}

	// Default plain text response
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
