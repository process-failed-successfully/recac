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
// It returns a mock response based on heuristics
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	lowerPrompt := strings.ToLower(prompt)

	// 1. Initializer / Architect Phase (Create Tickets)
	// Triggers on "json" AND ("technical program manager" OR "architect")
	// Prioritize this to ensure we return the plan before code.
	// But wait, the prompt usually contains the spec.
	if (strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "architect") || strings.Contains(lowerPrompt, "create exactly one ticket")) && strings.Contains(lowerPrompt, "json") {
		return `[
  {
    "id": "PRIMES",
    "type": "Task",
    "title": "Implement Prime Number Generator",
    "description": "Create a Python script that generates prime numbers up to a given limit.",
    "dependencies": [],
    "priority": "High"
  }
]`, nil
	}

	// 2. Commit Message Phase
	if strings.Contains(lowerPrompt, "commit message") {
		return "feat: Implement primes.py", nil
	}

	// 3. Coding Phase (Prime Generator)
	// Triggers on "primes" or "generate primes"
	// Ensure we don't trigger this if it's the TPM phase (though TPM check is above)
	if strings.Contains(lowerPrompt, "prime") || strings.Contains(lowerPrompt, "primes") || strings.Contains(lowerPrompt, "id:[primes]") {
		// Prevent infinite loop: if the code is already in the prompt (history), assume completion.
		if strings.Contains(lowerPrompt, "def generate_primes") {
			return "Task completed. The `primes.py` script has been created.", nil
		}
		return "```python\ndef generate_primes(n):\n    primes = []\n    for i in range(2, n + 1):\n        is_prime = True\n        for j in range(2, int(i ** 0.5) + 1):\n            if i % j == 0:\n                is_prime = False\n                break\n        if is_prime:\n            primes.append(i)\n    return primes\n\nif __name__ == '__main__':\n    import sys\n    limit = int(sys.argv[1]) if len(sys.argv) > 1 else 100\n    print(generate_primes(limit))\n```", nil
	}

	// Default fallback
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
