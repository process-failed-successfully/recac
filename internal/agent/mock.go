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

	// 1. Detect Prime Python Scenario (Ticket Generation Phase)
	// The prompt comes from JiraManager and contains the AppSpec with ID:[PRIMES]
	if strings.Contains(prompt, "ID:[PRIMES]") {
		// If prompt asks for JSON/Tickets (Ticket Generation Phase)
		// We can infer this if it's the large prompt with "Decompose the following application specification"
		// or just by the fact it's the first call in the scenario.
		// However, let's look for "ID:[PRIMES]" which is unique to the spec.
		// Return JSON array of tickets.
		return `[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.\n\nThe JSON format must have a single key 'primes' containing the list of integers.\nExample: {\"primes\": [2, 3, 5, ...]}",
    "type": "Task",
    "acceptance_criteria": [
      "Script primes.py exists",
      "primes.json is generated",
      "Contains exactly 1229 primes"
    ]
  }
]`, nil
	}

	// 2. Detect Prime Python Scenario (Implementation Phase)
	// The prompt comes from the Agent Loop and contains the ticket description.
	if strings.Contains(prompt, "Create Prime Number Script") || (strings.Contains(prompt, "primes.py") && strings.Contains(prompt, "primes.json")) {
		// Return a bash block to implement the solution.
		return `I will create the python script to calculate primes and save them to primes.json.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [n for n in range(2, 10000) if is_prime(n)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py
git add primes.py primes.json
git commit -m "Add primes.py and primes.json"
git push origin HEAD
` + "```" + `
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
