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
	callCount      int
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

	// Heuristic for Jira Generation (TPM Agent)
	// Trigger when prompt asks to generate tickets (usually contains "Technical Program Manager")
	if strings.Contains(prompt, "Technical Program Manager") {
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Create a Python script that generates prime numbers less than 10,000.",
    "type": "Task",
    "acceptance_criteria": [
      "Script is named primes.py",
      "Outputs to primes.json",
      "Contains a list of primes under the key 'primes'",
      "Exactly 1229 primes are generated"
    ],
    "children": []
  }
]`, nil
	}

	// Heuristic for Coding (Dev Agent) - Prime Python Scenario
	// Trigger when prompt asks for primes.py but is NOT the TPM prompt
	lowerPrompt := strings.ToLower(prompt)
	if (strings.Contains(lowerPrompt, "primes.py") || strings.Contains(lowerPrompt, "prime number")) && !strings.Contains(prompt, "Technical Program Manager") {
		m.callCount++
		if m.callCount == 1 {
			return `I will create the python script 'primes.py' that implements the Sieve of Eratosthenes to find all prime numbers less than 10,000 and outputs them to 'primes.json'.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def sieve(n):
    primes = []
    is_prime = [True] * n
    is_prime[0] = is_prime[1] = False
    for p in range(2, n):
        if is_prime[p]:
            primes.append(p)
            for i in range(p * p, n, p):
                is_prime[i] = False
    return primes

p = sieve(10000)
with open("primes.json", "w") as f:
    json.dump({"primes": p}, f)
EOF

python3 primes.py
git add primes.py primes.json
git commit -m "Add primes.py and primes.json"
` + "```" + `
`, nil
		}
		return "The script has been created and committed. Task Completed.", nil
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
