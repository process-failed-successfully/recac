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

	// Heuristics for Smoke Test Scenario (TPM)
	if strings.Contains(prompt, "You are an expert Technical Program Manager") {
		// Detect Primes Scenario
		if strings.Contains(prompt, "ID:[PRIMES]") {
			return `[
  {
    "title": "ID:[PRIMES] Prime Number Script",
    "description": "Create a python script named 'primes.py'. It must calculate all prime numbers less than 10,000 and output to a file named 'primes.json'. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Task",
    "children": []
  }
]`, nil
		}
	}

	// Heuristics for Initializer
	if strings.Contains(prompt, "YOUR ROLE - INITIALIZER AGENT") {
		return `
` + "```bash" + `
cat << 'EOF' | agent-bridge import
{
  "features": [
    {
      "id": "PRIMES",
      "description": "Calculate primes < 10000",
      "status": "todo"
    }
  ]
}
EOF
` + "```" + `
`, nil
	}

	// Heuristics for Coding Agent (Primes)
	if strings.Contains(prompt, "YOUR ROLE - CODING AGENT") && (strings.Contains(prompt, "PRIMES") || strings.Contains(prompt, "Prime")) {
		return `
I will implement the prime number script.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def get_primes(n):
    primes = []
    for i in range(2, n):
        is_prime = True
        for j in range(2, int(i**0.5) + 1):
            if i % j == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(i)
    return primes

primes = get_primes(10000)
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF
` + "```" + `

` + "```bash" + `
python3 primes.py
git add primes.py primes.json
git commit -m "Add primes script"
` + "```" + `

` + "```bash" + `
agent-bridge feature set PRIMES --status done --passes true
` + "```" + `
`, nil
	}

	// Heuristics for QA Agent
	if strings.Contains(prompt, "YOUR ROLE - QA AGENT") {
		return `
` + "```bash" + `
agent-bridge signal QA_PASSED true
` + "```" + `
`, nil
	}

	// Heuristics for Project Manager
	if strings.Contains(prompt, "YOUR ROLE - PROJECT MANAGER") {
		return `
` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
` + "```" + `
`, nil
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
