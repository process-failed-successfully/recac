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
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristics for different scenarios

	// 1. Technical Program Manager (Planning Phase) - Jira Ticket Generation
	// Trigger: "Technical Program Manager (TPM)" or "agile software development"
	if strings.Contains(prompt, "Technical Program Manager (TPM)") || strings.Contains(prompt, "agile software development") {
		// Return JSON array of ticketNode objects
		// This simulates the TPM breaking down the spec into Jira tickets
		// Includes Repo URL as required by validation/context
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Create a python script primes.py that calculates prime numbers up to 100 and saves them to primes.json.\n\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Story",
    "acceptance_criteria": [
      "Script runs without errors",
      "Output file primes.json contains valid JSON array of primes"
    ],
    "children": []
  }
]`, nil
	}

	// 2. Coding Agent (Execution Phase) - prime-python scenario
	// Trigger: "primes.py", "ID:[PRIMES]", or "1229" (common ID in tests)
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "1229") {
		// Return a bash script that implements the requirement and signals completion
		// Note: We include agent-bridge signal to mark the project as signed off
		return `cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [x for x in range(100) if is_prime(x)]
with open("primes.json", "w") as f:
    json.dump(primes, f)
EOF

python3 primes.py
git add primes.json primes.py
git commit -m "Add primes implementation" || echo "nothing to commit"
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
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
