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
	project        string
	model          string
}

// NewMockAgent creates a new mock agent
func NewMockAgent(apiKey, model, project string) *MockAgent {
	return &MockAgent{
		responsePrefix: "Mock agent response",
		project:        project,
		model:          model,
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

	promptUpper := strings.ToUpper(prompt)

	// Technical Program Manager Heuristic (JSON Tickets)
	if strings.Contains(promptUpper, "TECHNICAL PROGRAM MANAGER") && (strings.Contains(promptUpper, "TICKET") || strings.Contains(promptUpper, "PLAN")) {
		return `[
  {
    "title": "Implement Core Feature",
    "description": "Implement the core functionality based on the spec.",
    "type": "Epic"
  },
  {
    "title": "Implement Sub-task 1",
    "description": "First step of implementation.",
    "type": "Story",
    "blocked_by": ["Implement Core Feature"]
  }
]`, nil
	}

	// Coding Agent Heuristic (Primes Scenario)
	if strings.Contains(promptUpper, "CODING AGENT") && strings.Contains(promptUpper, "PRIMES") {
		return "I will implement the primes script.\n```python\ndef primes(n):\n    primes = []\n    for i in range(2, n + 1):\n        is_prime = True\n        for j in range(2, int(i ** 0.5) + 1):\n            if i % j == 0:\n                is_prime = False\n                break\n        if is_prime:\n            primes.append(i)\n    return primes\n\nif __name__ == '__main__':\n    import json\n    print(json.dumps(primes(100)))\n```", nil
	}

	// Initializer Agent Heuristic
	if strings.Contains(promptUpper, "INITIALIZER") || strings.Contains(promptUpper, "GET YOUR BEARINGS") {
		// Return a mock feature list import script
		return "I will initialize the feature list.\n```bash\nagent-bridge import --file features.json\n```", nil
	}

	// QA Agent Heuristic
	if strings.Contains(promptUpper, "QA AGENT") {
		return "I have verified the changes.\n```bash\nagent-bridge signal QA_PASSED true\n```", nil
	}

	// Project Manager Heuristic
	if strings.Contains(promptUpper, "PROJECT MANAGER") {
		return "I approve the project.\n```bash\nagent-bridge signal PROJECT_SIGNED_OFF true\n```", nil
	}

	// Default response
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
