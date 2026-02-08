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

	// [PRIMES] Scenario Heuristics

	// 1. TPM Role (Ticket Generation)
	// The `recac jira generate-from-spec` command sends a prompt asking to break down the spec.
	// We must return valid JSON for the CLI to parse.
	if (strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM")) && strings.Contains(prompt, "[PRIMES]") {
		return `[
  {
    "title": "ID:[PRIMES] Create primes.py",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to 'primes.json'. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Task",
    "acceptance_criteria": [
      "primes.py exists",
      "primes.json is generated",
      "Contains 1229 primes",
      "Primes are < 10000"
    ],
    "children": []
  }
]`, nil
	}

	// 2. Coding Agent Role (Implementation)
	// The orchestrator loop sends a prompt to implement the ticket.
	// We must return a conversational response with a bash block.
	if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py") {
		return m.handlePrimesCodingScenario(prompt), nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func (m *MockAgent) handlePrimesCodingScenario(prompt string) string {
	// If the prompt asks for the primes script, generate it.
	// We return a response that creates the file using a bash block, runs it, and commits it.

	return `I will create the 'primes.py' script to calculate prime numbers less than 10,000 and output them to 'primes.json'.

Here is the plan:
1. Create 'primes.py' with the prime calculation logic.
2. Run 'primes.py' to generate 'primes.json'.
3. Commit the changes.
4. Signal completion.

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

primes = [n for n in range(10000) if is_prime(n)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py

git add primes.py primes.json
git commit -m "Add primes script and output"

agent-bridge feature set PRIMES --status done --passes true
` + "```" + `
`
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
