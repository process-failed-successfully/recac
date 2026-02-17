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

	// Heuristics for E2E tests

	// 1. Planning Phase (TPM)
	// The prompt for generating tickets usually comes from TPM Agent and asks for JSON
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "Output purely JSON") {
		// Return JSON plan for Primes (matching the prime-python scenario expectation)
		// We return a list of tickets as expected by generateTickets
		return `
[
  {
    "title": "ID:[PRIMES] Prime Number Generator",
    "description": "Implement a Python script to generate prime numbers less than 10,000 and save them to primes.json.",
    "type": "Story",
    "acceptance_criteria": [
      "Script primes.py generates primes < 10000",
      "Output is saved to primes.json in JSON format {\"primes\": [...]}"
    ],
    "children": []
  }
]`, nil
	}

	// 2. Execution Phase (Coding Agent) - Primes
	// The prompt for implementation usually asks to implement the task
	if strings.Contains(prompt, "primes") || strings.Contains(prompt, "Prime Number Generator") {
		return `Here is the solution to generate primes < 10,000.

` + "```bash" + `
# Configure git if needed (mock environment)
git config user.email "agent@recac.com" || true
git config user.name "Recac Agent" || true

cat << 'EOF' > primes.py
import json

def get_primes(n):
    primes = []
    for num in range(2, n):
        is_prime = True
        for i in range(2, int(num**0.5) + 1):
            if num % i == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(num)
    return primes

primes = get_primes(10000)
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
print(f"Generated {len(primes)} primes.")
EOF

# Run the script
python3 primes.py

# Commit
git add primes.py primes.json
git commit -m "Add primes generation script" || echo "Nothing to commit"
` + "```", nil
	}

	// 3. QA Agent or other signals
	if strings.Contains(prompt, "QA AGENT") {
		return `
` + "```bash" + `
agent-bridge signal QA_PASSED true
` + "```", nil
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
