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

	// --- Heuristics ---

	// 1. Completion Heuristic (Must be first to prevent loops)
	if strings.Contains(lowerPrompt, "nothing to commit") || strings.Contains(lowerPrompt, "no changes") {
		return "Done.", nil
	}

	// 2. Technical Program Manager (TPM) / Ticket Generation Heuristic
	// Used by `recac` CLI during `generate-from-spec`
	if (strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "tpm")) &&
		(strings.Contains(lowerPrompt, "json") || strings.Contains(lowerPrompt, "ticket")) {
		return `[
  {
    "title": "ID:[PRIMES] Implement Python Script",
    "description": "Create a Python script that generates prime numbers.",
    "type": "Task",
    "children": []
  },
  {
    "title": "ID:[QA] Verify Python Script",
    "description": "Run the Python script and verify output.",
    "type": "Task",
    "children": []
  }
]`, nil
	}

	// 3. Architect / Project Plan Heuristic
	if strings.Contains(lowerPrompt, "feature list") || strings.Contains(lowerPrompt, "break down") ||
		strings.Contains(lowerPrompt, "decompose") || strings.Contains(lowerPrompt, "application specification") {
		return `{
  "project_name": "Mock Project",
  "technologies": ["Go", "Python"],
  "features": [
    {"name": "Core Logic", "description": "Implement core algorithms"},
    {"name": "API", "description": "REST API interface"}
  ]
}`, nil
	}

	// 4. Coding / Implementation Heuristic (Primes Scenario)
	if strings.Contains(lowerPrompt, "prime number") || strings.Contains(lowerPrompt, "primes.json") ||
		strings.Contains(lowerPrompt, "id:[primes]") || strings.Contains(lowerPrompt, "generate primes") {
		return "```python\nimport json\n\ndef generate_primes(n):\n    primes = []\n    num = 2\n    while len(primes) < n:\n        is_prime = True\n        for i in range(2, int(num ** 0.5) + 1):\n            if num % i == 0:\n                is_prime = False\n                break\n        if is_prime:\n            primes.append(num)\n        num += 1\n    return primes\n\nif __name__ == '__main__':\n    data = {'primes': generate_primes(10)}\n    with open('primes.json', 'w') as f:\n        json.dump(data, f)\n    print('Generated primes.json')\n```", nil
	}

    // 5. Manager Heuristic
    if strings.Contains(lowerPrompt, "manager") || strings.Contains(lowerPrompt, "lead software architect") {
        return "Approving the plan. Proceed with implementation.", nil
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
	return s[:maxLen] + "..."
}
