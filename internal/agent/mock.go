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

	lowerPrompt := strings.ToLower(prompt)

	// 1. Initializer Agent
	// Returns a JSON list of features for the "primes" scenario
	if strings.Contains(lowerPrompt, "feature_list.json") {
		return `[
  {
    "id": "PRIMES",
    "category": "core",
    "priority": "high",
    "status": "todo",
    "dependencies": [],
    "title": "Implement Primes",
    "description": "Implement a Python script to find prime numbers."
  }
]`, nil
	}

	// 2. Coding Agent (Implementation)
	// If asked to implement primes, generate the python script and signal completion
	if strings.Contains(lowerPrompt, "primes") {
		// If we already see the file exists or nothing to commit, we are done
		if strings.Contains(lowerPrompt, "nothing to commit") || strings.Contains(lowerPrompt, "working tree clean") {
			return `
echo "Task verified. Implementation complete."
agent-bridge signal COMPLETED
`, nil
		}

		return `
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

if __name__ == "__main__":
    for i in range(20):
        if is_prime(i):
            print(i)
EOF

# Run it to verify
python3 primes.py

# Commit
git add primes.py
git commit -m "Implement primes.py" || echo "Nothing to commit"

# Signal completion
agent-bridge signal COMPLETED
`, nil
	}

	// 3. QA Agent
	if strings.Contains(lowerPrompt, "qa agent") {
		return `
echo "QA Passed."
agent-bridge signal QA_PASSED
`, nil
	}

	// 4. Manager Agent
	if strings.Contains(lowerPrompt, "project manager") {
		return `
echo "Project Signed Off."
agent-bridge signal PROJECT_SIGNED_OFF
`, nil
	}

    // Default Fallback
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
