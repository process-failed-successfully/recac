package agent

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// MockAgent is a smart mock agent for E2E testing
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

// Send implements the Agent interface with heuristic logic for E2E scenarios
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// 1. TPM Role (Planning) - Detects TPM prompt
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") {
		// Return JSON array of tickets as expected by 'recac jira generate-from-spec'
		return `[
  {
    "title": "ID:[PRIMES] Implement Primes",
    "description": "Implement a Python script to calculate prime numbers up to 10,000.",
    "status": "todo",
    "type": "task"
  }
]`, nil
	}

	// 2. Coding Agent Role (Implementation) - Detects 'primes' task
	if strings.Contains(prompt, "prime") || strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "[PRIMES]") {
		// Return bash script to create primes.py and run it
		// Escape % as %% is handled if we were using printf, but here it's raw string.
		// NOTE: Script must generate primes.json as per E2E requirement
		return `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(10000) if is_prime(x)]
with open('primes.json', 'w') as f:
    json.dump({'primes': primes}, f)
print(f"Generated primes.json with {len(primes)} primes")
EOF

python3 primes.py
# Mark the feature as completed
agent-bridge feature update PRIMES --status completed
`, nil
	}

	// 3. QA/Manager Role (Review/Sign-off)
	if strings.Contains(prompt, "QA") || strings.Contains(prompt, "Review") || strings.Contains(prompt, "Manager") {
		return `
agent-bridge signal PROJECT_SIGNED_OFF true --privileged || touch PROJECT_SIGNED_OFF
echo "QA Passed. Project signed off."
`, nil
	}

	// Default Fallback
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

// SendStream implements the Agent interface
func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	// Simulate slight delay to prevent tight loops in tests
	time.Sleep(10 * time.Millisecond)

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
