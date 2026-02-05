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

	// Heuristic for Technical Program Manager (TPM) - Prioritize over general keyword checks
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") || strings.Contains(prompt, "Application Specification") {
		if strings.Contains(prompt, "[PRIMES]") {
			return m.handlePrimesTPMScenario()
		}
	}

	// Heuristic for E2E Prime Python Scenario (Coding Phase)
	if strings.Contains(prompt, "[PRIMES]") || (strings.Contains(prompt, "primes.py") && strings.Contains(prompt, "python")) {
		return m.handlePrimesScenario()
	}

	// Heuristic for Manager Agent (if it picks it up)
	// If the prompt looks like a Jira ticket spec or asks for a plan, return a simple plan or "QA_PASSED" if it's verifying.
	if strings.Contains(prompt, "QA_PASSED") {
		return "QA_PASSED", nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func (m *MockAgent) handlePrimesTPMScenario() (string, error) {
	// Return a JSON list of tickets
	json := `[
  {
    "id": "PRIMES",
    "title": "Implement Primes Script",
    "description": "Create a python script that calculates primes up to 10000 and saves them to primes.json. The script should be named primes.py.",
    "type": "Task",
    "status": "To Do",
    "assignee": "recac-agent"
  }
]`
	return json, nil
}

func (m *MockAgent) handlePrimesScenario() (string, error) {
	// Generate the solution for the prime-python scenario
	script := `
git config user.email "mock@agent.com"
git config user.name "Mock Agent"

cat << 'EOF' > primes.py
import json

primes = []
for num in range(2, 10000):
    is_prime = True
    for i in range(2, int(num ** 0.5) + 1):
        if num % i == 0:
            is_prime = False
            break
    if is_prime:
        primes.append(num)

with open('primes.json', 'w') as f:
    json.dump({'primes': primes}, f)
EOF

python3 primes.py

git add primes.py primes.json
git commit -m "Add primes script and output"
git push origin HEAD || echo "Push skipped"
`
	return fmt.Sprintf("I will create the primes script as requested.\n\n```bash\n%s\n```", script), nil
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
