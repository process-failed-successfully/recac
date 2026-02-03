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

	// Smart Mock Logic for Smoke Tests
	// 1. Check for Prime Number Scenario
	if strings.Contains(strings.ToLower(prompt), "primes.py") || strings.Contains(strings.ToLower(prompt), "prime number script") {
		// Return a response that creates the python file and the json output
		return `
I will implement the prime number script.

<file path="primes.py">
import json

def get_primes(n):
    primes = []
    for i in range(2, n):
        is_prime = True
        for j in range(2, int(i ** 0.5) + 1):
            if i % j == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(i)
    return primes

primes = get_primes(10000)
with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)
print(f"Found {len(primes)} primes")
</file>

I will also create the output file directly to ensure it exists.

<command>
python3 primes.py
git add primes.py primes.json
git commit -m "Add primes.py and primes.json"
</command>

PROJECT_SIGNED_OFF
`, nil
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
