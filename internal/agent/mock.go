package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// MockAgent is a simple mock agent for testing and mock mode
// It returns predefined responses without making actual API calls
type MockAgent struct {
	responsePrefix string
	forcedResponse string
	callCount      int
	mu             sync.Mutex
}

// NewMockAgent creates a new mock agent
func NewMockAgent() *MockAgent {
	return &MockAgent{
		responsePrefix: "Mock agent response",
	}
}

// SetResponse forces a specific response from the agent
func (m *MockAgent) SetResponse(response string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.forcedResponse = response
}

// Send implements the Agent interface
// It returns a mock response that acknowledges the prompt
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	lowerPrompt := strings.ToLower(prompt)

	// Heuristic for E2E Smoke Test (Primes)
	// Triggers when the prompt asks to implement the primes generator
	if strings.Contains(lowerPrompt, "prime") || strings.Contains(lowerPrompt, "primes.py") {
		m.callCount++

		// Step 1: Create the file and run it
		if m.callCount == 1 {
			return `I will create the prime number generator script and run it.

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

# Generate primes up to 10000 (which contains exactly 1229 primes)
primes = [x for x in range(1, 10000) if is_prime(x)]

# Requirement: Check count
if len(primes) != 1229:
    print(f"Error: Expected 1229 primes, got {len(primes)}")
    exit(1)

output = {"primes": primes}
with open('primes.json', 'w') as f:
    json.dump(output, f)

print(f"Generated {len(primes)} primes to primes.json")
EOF

python3 primes.py
` + "```", nil
		}

		// Step 2: Commit and Signal Completion
		if m.callCount >= 2 {
			return `The task is complete. I will commit the results and update the status.

` + "```bash" + `
git add primes.py primes.json
git commit -m "feat: Implement prime number generator"

# Update status for all requirements (found in logs)
agent-bridge feature set req-script-is-named-primes-py --status done --passes true
agent-bridge feature set req-outputs-to-primes-json --status done --passes true
agent-bridge feature set req-contains-a-list-of-primes-unde --status done --passes true
agent-bridge feature set req-exactly-1229-primes-are-genera --status done --passes true

# Signal completion
agent-bridge signal COMPLETED true
` + "```", nil
		}
	}

	// Heuristic for Technical Program Manager (TPM)
	if strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "plan in json") {
		return `[
  {"id": "TASK-1", "type": "Task", "title": "Implement Primes", "description": "Implement a prime number generator in Python that outputs to primes.json", "dependencies": []}
]`, nil
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
