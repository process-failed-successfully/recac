package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a smart mock agent for testing and mock mode
// It returns predefined responses based on heuristics to simulate agent behavior
type MockAgent struct {
	responsePrefix string
	forcedResponse string
	hasCommitted   bool
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

	// Heuristic: Initializer (Import Features)
	if strings.Contains(prompt, "You are the Initializer") || strings.Contains(prompt, "INITIALIZER AGENT") {
		return `
` + "```bash" + `
cat <<EOF > feature_list.json
{
  "features": [
    {"id": "req-primes", "description": "Implement primes.py", "priority": "MVP", "status": "pending"}
  ]
}
EOF
agent-bridge import feature_list.json
` + "```" + `
`, nil
	}

	// Heuristic: Technical Program Manager (Generate Tickets)
	if strings.Contains(prompt, "Technical Program Manager") {
		return `[{"id": "PRIMES", "key": "PRIMES", "title": "Implement Primes", "description": "Implement primes.py", "type": "Task"}]`, nil
	}

	// Heuristic: Project Manager (Sign Off)
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return `
` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true
` + "```" + `
`, nil
	}

	// Heuristic: Coding Agent (Primes Scenario)
	// Check case-insensitive "primes" to catch "Primes", "PRIMES", "primes"
	// Also fallback to this behavior if "CODING AGENT" is present, as it's the primary mock scenario.
	if strings.Contains(strings.ToLower(prompt), "primes") || strings.Contains(prompt, "Prime Number Script") || strings.Contains(prompt, "CODING AGENT") {
		if !m.hasCommitted {
			m.hasCommitted = true
			return `
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

python3 primes.py
git add primes.py primes.json
git commit -m "Implement primes.py and generate primes.json"
` + "```" + `
`, nil
		}
		// If already committed, signal success to break loop
		return `
` + "```bash" + `
agent-bridge signal QA_PASSED true
` + "```" + `
`, nil
	}

	// Heuristic: QA Agent
	if strings.Contains(prompt, "QA AGENT") {
		return `
` + "```bash" + `
agent-bridge signal QA_PASSED true
` + "```" + `
`, nil
	}

	// Default response
	return fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100)), nil
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
