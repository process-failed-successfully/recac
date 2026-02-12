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
// It returns a mock response based on heuristics or acknowledges the prompt
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	lowerPrompt := strings.ToLower(prompt)

	// Heuristic: Initializer Agent (Feature Extraction)
	if strings.Contains(lowerPrompt, "initializer agent") || strings.Contains(lowerPrompt, "feature extraction") {
		return `I have analyzed the requirements. Here are the features:

` + "```bash" + `
echo '[{"id": "PRIMES", "description": "Calculate prime numbers less than 10,000", "status": "pending"}]' | agent-bridge import --project "$RECAC_PROJECT_ID"
` + "```" + `

Features imported.
`, nil
	}

	// Heuristic: Coding Agent (Prime Python Scenario)
	if (strings.Contains(lowerPrompt, "prime") && strings.Contains(lowerPrompt, "python")) || (strings.Contains(lowerPrompt, "primes") && strings.Contains(lowerPrompt, "script")) {
		return `I will create a Python script to calculate prime numbers.

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

primes = [i for i in range(10000) if is_prime(i)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py
git add primes.py primes.json
git commit -m "Add primes.py and primes.json"
agent-bridge feature set PRIMES --status completed --passes true
` + "```" + `

I have implemented the script and committed it.
`, nil
	}

	// Heuristic: QA Agent
	if strings.Contains(lowerPrompt, "qa") || strings.Contains(lowerPrompt, "review") || strings.Contains(lowerPrompt, "verify") {
		return `QA Check: Passed.

The code looks good and meets the requirements.

` + "```bash" + `
agent-bridge signal QA_PASSED
` + "```" + `
`, nil
	}

	// Heuristic: Project Manager (Sign Off)
	if strings.Contains(lowerPrompt, "manager") || strings.Contains(lowerPrompt, "sign off") {
		return `Project is complete.

` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF --privileged
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
