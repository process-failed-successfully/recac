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

	// Heuristics for E2E Tests
	lowerPrompt := strings.ToLower(prompt)

	// 1. Technical Program Manager (TPM) - Planning Phase
	// Used by 'recac generate-from-spec'
	if strings.Contains(lowerPrompt, "technical program manager (tpm)") ||
		strings.Contains(lowerPrompt, "agile software development") ||
		strings.Contains(lowerPrompt, "generate jira tickets") {
		// Return JSON array of tickets for Prime Python scenario
		return `[
  {
    "title": "Implement Prime Number Script",
    "description": "Create a python script 'primes.py' that calculates primes up to 100 and saves them to 'primes.json'.",
    "type": "Task",
    "id": "PRIMES-1",
    "requirements": ["req-script-runs-without-errors"],
    "children": []
  }
]`, nil
	}

	// 2. Coding Agent - Execution Phase
	// Used by the agent loop (Docker/K8s) to execute the task
	if strings.Contains(lowerPrompt, "primes.py") ||
		strings.Contains(lowerPrompt, "id:[primes]") ||
		strings.Contains(lowerPrompt, "1229") || // Magic number sometimes used
		strings.Contains(lowerPrompt, "implement prime number script") {

		return `
# Create primes.py
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [n for n in range(2, 101) if is_prime(n)]

with open('primes.json', 'w') as f:
    json.dump(primes, f)
EOF

# Run script
python3 primes.py

# Verify output
if [ ! -f primes.json ]; then
    echo "primes.json not found"
    exit 1
fi

# Commit
git add primes.py primes.json
git commit -m "Implement primes script" || echo "nothing to commit"

# Signal Completion
agent-bridge import --requirement req-script-runs-without-errors --description "Script runs without errors"
agent-bridge feature set req-script-runs-without-errors completed
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
`, nil
	}

	// Default Mock Response
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
