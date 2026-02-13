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

	// Heuristic 1: Initializer Agent (Feature Import)
	if strings.Contains(strings.ToLower(prompt), "initializer agent") {
		return "```bash\necho '{\"features\": [{\"id\": \"prime-script\", \"description\": \"Implement prime script\", \"status\": \"pending\"}]}' | agent-bridge import\n```", nil
	}

	// Heuristic 2: TPM Agent (Ticket Generation)
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "ID:[PRIMES]") {
		// Return JSON for the ticket
		return `[
  {
    "title": "ID:[PRIMES] Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.",
    "type": "Task",
    "children": []
  }
]`, nil
	}

	// Heuristic 3: Coding Agent (Primes Implementation)
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "PRIMES") {
		return `Here is the python script to calculate primes:

` + "```python" + `
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [x for x in range(10000) if is_prime(x)]
with open('primes.json', 'w') as f:
    json.dump({'primes': primes}, f)
` + "```" + `

And the command to run and commit:

` + "```bash" + `
python3 primes.py
git add -f primes.py primes.json
git commit -m "feat: add primes script"
` + "```", nil
	}

	// Default Response
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
