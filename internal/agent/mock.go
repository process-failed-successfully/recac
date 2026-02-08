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

	// Heuristics

	// 1. Initializer Agent
	if strings.Contains(prompt, "ROLE - INITIALIZER AGENT") {
		return `
cat <<EOF > feature_list.json
[
  {
    "id": "req-primes-implementation",
    "title": "Primes Implementation",
    "description": "Implement primes.py",
    "type": "Task"
  }
]
EOF
cat feature_list.json | agent-bridge import
`, nil
	}

	// 2. Technical Program Manager (TPM)
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "ROLE - PROJECT MANAGER") {
		// If it's about PRIMES
		if strings.Contains(prompt, "[PRIMES]") {
			return `
ID:[PRIMES] Create primes.py
Type: Task
Description: Implement primes.py to calculate primes < 10000.
`, nil
		}
	}

	// 3. Loop Breaker
	if strings.Contains(prompt, "nothing to commit") || strings.Contains(prompt, "working tree clean") {
		id := "req-primes-implementation"
		if strings.Contains(prompt, "[PRIMES]") {
			id = "PRIMES" // Use generic ID if prompt suggests
		}
		// If the prompt has req-primes-implementation use that
		if strings.Contains(prompt, "req-primes-implementation") {
			id = "req-primes-implementation"
		}
		return fmt.Sprintf(`
agent-bridge feature set %s --status done --passes true
agent-bridge signal COMPLETED true
`, id), nil
	}

	// 4. Coding Agent (Implementation)
	if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "req-primes-implementation") {
		return `
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

git add primes.py primes.json
git commit -m "Implement primes" || echo "Nothing to commit"

agent-bridge feature set req-primes-implementation --status done --passes true
`, nil
	}

	// 5. QA Agent
	if strings.Contains(prompt, "QA AGENT") || strings.Contains(prompt, "verify the project") {
		return `
agent-bridge signal QA_PASSED true
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
