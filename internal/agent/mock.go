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

	// Priority 1: Check if this is the "Prime Python" scenario implementation phase.
	// We check this BEFORE ticket generation because the prompt might contain "app_spec.txt" context,
	// which would incorrectly trigger the ticket generation response if checked first.
	if strings.Contains(prompt, "primes.py") {
		return "Here is the implementation for primes.py:\n\n```bash\ncat << 'EOF' > primes.py\nimport json\n\ndef is_prime(n):\n    if n <= 1: return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0: return False\n    return True\n\nprimes = [x for x in range(10000) if is_prime(x)]\nwith open('primes.json', 'w') as f:\n    json.dump({\"primes\": primes}, f)\nEOF\n\npython3 primes.py\ngit add primes.py\ngit add -f primes.json\ngit commit -m \"Add primes script and output\"\n```", nil
	}

	// Priority 2: Check if this is a ticket generation request (based on prompt content or context)
	// The smoke test sends a prompt with the app spec.
	if strings.Contains(prompt, "app_spec.txt") || strings.Contains(prompt, "Technical Program Manager") {
		// Return a valid JSON response for the ticket generator
		// We deliberately include "primes.py" in the description to trigger the implementation logic in the next turn
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Implement the primes.py script to generate prime numbers as requested.",
    "type": "Task",
    "acceptance_criteria": [
      "primes.py is created",
      "primes.json is generated"
    ],
    "children": []
  }
]`, nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	// IMPORTANT: We include a no-op bash block to prevent the agent runner's circuit breaker
	// from tripping due to "no commands executed" loops in mock mode.
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...\n\n```bash\n# no-op to satisfy circuit breaker\n```",
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
