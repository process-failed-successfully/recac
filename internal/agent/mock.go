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

	// Heuristic: Check if the prompt is for the TPM / Architect agent (Generate Tickets)
	// The prompt typically starts with "You are an expert Technical Program Manager..."
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "generate ticket plan") {
		// Return a valid JSON plan for the 'prime-python' scenario or generic usage
		return `[
  {
    "title": "ID:[PRIMES] Prime Number Generator",
    "description": "Implement a Python script to generate prime numbers.",
    "type": "Story",
    "children": [],
    "acceptance_criteria": [
      "Must generate primes up to N",
      "Must output result to primes.json"
    ]
  }
]`, nil
	}

	// Heuristic: Check if the prompt is for the Coding Agent (Implement Code)
	// The prompt typically asks to "implement the following task" or similar
	if strings.Contains(prompt, "Implement a Python script") || strings.Contains(strings.ToLower(prompt), "primes") {
		// Return a valid Python script for the 'prime-python' scenario
		return "```python\nimport json\n\ndef generate_primes(n):\n    primes = []\n    for num in range(2, n + 1):\n        is_prime = True\n        for i in range(2, int(num ** 0.5) + 1):\n            if num % i == 0:\n                is_prime = False\n                break\n        if is_prime:\n            primes.append(num)\n    return primes\n\nif __name__ == '__main__':\n    primes = generate_primes(100)\n    with open('primes.json', 'w') as f:\n        json.dump({'primes': primes}, f)\n    print(f'Generated {len(primes)} primes.')\n```", nil
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
