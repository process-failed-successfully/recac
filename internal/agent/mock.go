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

	// Heuristic for E2E Prime Python Scenario
	if strings.Contains(prompt, "[PRIMES]") || (strings.Contains(prompt, "primes.py") && strings.Contains(prompt, "calculate")) {
		// 1. TPM Agent (Ticket Generation)
		if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") {
			return m.generatePrimesPlan(), nil
		}
		// 2. Coding Agent (Implementation)
		// If the history indicates we've already generated the script, signal completion to avoid loops.
		if strings.Contains(prompt, "cat << 'EOF' > primes.py") {
			return m.generatePrimesCompletion(), nil
		}
		// Default to implementation if role is ambiguous but scenario is clearly primes (legacy behavior support)
		return m.generatePrimesResponse(), nil
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

func (m *MockAgent) generatePrimesResponse() string {
	script := `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [i for i in range(10000) if is_prime(i)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py
git add primes.py primes.json
git commit --author="Sentinel <sentinel@recac.com>" -m "Add primes script and output" || echo "Nothing to commit"
git push origin HEAD || echo "Push skipped"
`
	return fmt.Sprintf("I will implement the prime number script as requested.\n\n```bash%s```", script)
}

func (m *MockAgent) generatePrimesCompletion() string {
	return `The prime number script has been implemented and executed successfully.
I will now mark the feature as complete and signal the task is finished.

` + "```bash" + `
agent-bridge feature set req-script-calculates-primes-corre --status done --passes true
agent-bridge feature set req-output-is-saved-to-primes-json --status done --passes true
agent-bridge feature set req-contains-exactly-1229-primes --status done --passes true
agent-bridge signal COMPLETED true
` + "```"
}

func (m *MockAgent) generatePrimesPlan() string {
	return `
[
  {
    "title": "ID:[PRIMES] Prime Number Script",
    "description": "Implement a Python script that calculates prime numbers up to 10,000 and saves them to a JSON file.",
    "type": "Task",
    "acceptance_criteria": [
      "Script calculates primes correctly",
      "Output is saved to primes.json",
      "Contains exactly 1229 primes"
    ]
  }
]
`
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
