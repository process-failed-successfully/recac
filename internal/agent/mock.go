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

	// TPM Planning Phase (generate-from-spec)
	if strings.Contains(prompt, "Technical Program Manager (TPM)") || strings.Contains(prompt, "agile software development") {
		// Return JSON tickets for the prime-python scenario or generic task
		return `[
  {
    "title": "ID:[PRIMES] Implement Primes Python Script",
    "description": "Implement a Python script that generates prime numbers.",
    "type": "Story",
    "children": [],
    "blocked_by": [],
    "acceptance_criteria": [
      "Script runs without errors",
      "Writes prime numbers to primes.json"
    ]
  }
]`, nil
	}

	// QA Agent
	if strings.Contains(prompt, "YOUR ROLE - QA AGENT") {
		return "All tests passed", nil
	}

	// Manager Review
	if strings.Contains(prompt, "YOUR ROLE - PROJECT MANAGER") {
		return "Approve", nil
	}

	// Coding Agent (Prime Python Scenario)
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "1229") {
		return `
cat <<EOF > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(100) if is_prime(x)]
with open('primes.json', 'w') as f:
    json.dump(primes, f)
print(f"Generated {len(primes)} primes")
EOF

python3 primes.py
git add primes.py primes.json
git commit -m "Implement primes" || echo "nothing to commit"
git push || echo "nothing to push"

# Signal completion
agent-bridge import --id "ID:[PRIMES]" --requirement "Script runs without errors" --status completed
agent-bridge import --id "ID:[PRIMES]" --requirement "Writes prime numbers to primes.json" --status completed
agent-bridge feature set --id "ID:[PRIMES]" --status completed
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
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
