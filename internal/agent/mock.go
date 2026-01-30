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
// It also has logic to handle specific prompts for the smoke test scenario
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Smoke Test / Prime Python Scenario Logic

	// 1. Ticket Generation Phase (Plan)
	// Check for "Technical Program Manager" OR ("ID:[PRIMES]" AND ("Ticket" OR "Jira"))
	isPlanPhase := strings.Contains(prompt, "Technical Program Manager") ||
		(strings.Contains(prompt, "ID:[PRIMES]") && (strings.Contains(prompt, "Ticket") || strings.Contains(prompt, "Jira")))

	if isPlanPhase {
		// Return JSON for the ticket generation phase
		return `[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Implement primes.py to calculate primes < 10000. Output to primes.json.",
    "type": "Task",
    "children": []
  }
]`, nil
	}

	// 2. Implementation Phase
	// If it didn't match the Plan phase, check for implementation keywords
	if strings.Contains(prompt, "PRIMES") || strings.Contains(prompt, "primes.py") {
		// Return bash script for implementation phase
		return `Here is the script to generate primes:

` + "```bash" + `
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
    json.dump({"primes": primes}, f)
EOF

python3 primes.py
git add primes.json
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
