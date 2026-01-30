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

	// Heuristic for Ticket Generation (TPM Agent) used in E2E tests
	if (strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "app_spec.txt")) && strings.Contains(prompt, "JSON") {
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Implement a Python script to generate prime numbers.",
    "type": "Story",
    "acceptance_criteria": [
      "Script runs successfully",
      "Generates primes up to N"
    ],
    "children": [
      {
        "title": "ID:[PRIMES-IMP] Implement Sieve",
        "description": "Implement the Sieve of Eratosthenes in primes.py",
        "type": "Subtask"
      }
    ]
  }
]`, nil
	}

	// Heuristic for Implementation (Developer Agent) used in E2E tests
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "Prime Number Generator") {
		return "```bash\ncat <<EOF > primes.py\ndef is_prime(n):\n    if n <= 1: return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0:\n            return False\n    return True\n\nimport sys\nif __name__ == '__main__':\n    print([x for x in range(int(sys.argv[1])) if is_prime(x)])\nEOF\n```", nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	// We include a trivial bash comment block to prevent the session circuit breaker
	// from tripping due to "no executable commands found".
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...\n\n```bash\n# Mock command to satisfy circuit breaker\nls -la\n```",
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
