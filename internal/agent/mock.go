package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a smart mock agent for testing and mock mode
// It uses heuristics to return useful responses for known scenarios (like E2E tests)
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

	// Heuristics for E2E Scenarios

	// 1. Prime Python Scenario
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "[PRIMES]") {
		// Check if it's already implemented (to avoid loops)
		if strings.Contains(prompt, "git status") && strings.Contains(prompt, "primes.json") {
			// If we see evidence of work done, we might want to say "Task Complete"
			// But usually the prompt comes from the runner.
			// Let's just output the solution.
		}

		return `I will implement the prime number script as requested.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [x for x in range(10000) if is_prime(x)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF
` + "```" + `

Now I will run the script and commit the results.

` + "```bash" + `
python3 primes.py
git add primes.py primes.json
git commit -m "Add primes script and output" || echo "No changes to commit"
` + "```" + `
`, nil
	}

	// 2. Default/Fallback
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))

	// Add a safe command to prevent "NO-OP LOOP" errors in some runners
	response += "\n\n```bash\necho \"Mock Agent Default Response\"\n```"

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
