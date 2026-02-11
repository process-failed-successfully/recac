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
[{"id": "req-primes", "description": "Implement primes.py"}]
EOF
# Fallback: agent-bridge is removed to prevent failure in unit tests.
# The runner will automatically pick up feature_list.json from disk.
` + "```" + `
`, nil
	}

	// Heuristic: Technical Program Manager (Generate Tickets)
	if strings.Contains(prompt, "Technical Program Manager") {
		return `
` + "```json" + `
[{"id": "PRIMES", "key": "PRIMES", "title": "Implement Primes", "description": "Implement primes.py", "type": "Task"}]
` + "```" + `
`, nil
	}

	// Heuristic: Project Manager (Sign Off)
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return `
` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true || touch PROJECT_SIGNED_OFF
` + "```" + `
`, nil
	}

	// Heuristic: Coding Agent (Primes Scenario)
	// Check case-insensitive to ensure robustness (e.g. "Implement Primes")
	promptLower := strings.ToLower(prompt)
	if strings.Contains(promptLower, "primes") || strings.Contains(promptLower, "prime number script") {
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
agent-bridge signal QA_PASSED true || touch QA_PASSED
` + "```" + `
`, nil
	}

	// Heuristic: QA Agent or Generic Ticket (Fallback for unit tests)
	// Triggered by "QA AGENT" or general "Ticket" references if no specific heuristic matched
	if strings.Contains(prompt, "QA AGENT") || strings.Contains(prompt, "Ticket") {
		return `
` + "```bash" + `
agent-bridge signal QA_PASSED true || touch QA_PASSED
` + "```" + `
`, nil
	}

	// Default response - Must include a no-op command to prevent "No-Op Loop" errors in strict mode
	return fmt.Sprintf(`%s: I received your prompt (%d chars).
`+"```bash"+`
echo "Mock Agent processing prompt..."
`+"```"+`
`, m.responsePrefix, len(prompt)), nil
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
