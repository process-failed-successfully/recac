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

	// Smart Mock Logic for Smoke Tests

	// 1. Ticket Generation Phase (Check this FIRST)
	// The prompt from PrimePythonScenario.AppSpec contains "CRITICAL INSTRUCTION FOR TICKET GENERATION"
	// and typically asks to generate tickets based on the spec.
	if strings.Contains(lowerPrompt, "critical instruction for ticket generation") {
		// Return a valid JSON response for ticket generation
		return `
{
  "tickets": [
    {
      "title": "ID:[PRIMES] Create Prime Number Script",
      "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to 'primes.json'.\n\nREQUIRED FEATURES:\n- Implement prime calculation logic in primes.py\n- Output results to primes.json\n- Validate that the output file contains a 'primes' list\n- Verify that exactly 1229 primes are calculated\n- Commit primes.json to the repository",
      "type": "Task"
    }
  ]
}
`, nil
	}

	// 2. Implementation Phase
	// Check for "primes.py" or "prime number script" BUT ensure it's not the AppSpec itself.
	// The AppSpec prompt is usually long and contains "ID:[PRIMES]". The implementation prompt
	// sent by the agent loop is usually focused on the specific task.
	// However, the safest way is simply the order: we already handled the Ticket Generation prompt above.
	// So if we are here, it might be the implementation prompt.
	if strings.Contains(lowerPrompt, "primes.py") || strings.Contains(lowerPrompt, "prime number script") {
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
