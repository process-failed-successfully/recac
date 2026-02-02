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

	upperPrompt := strings.ToUpper(prompt)

	// Feature List Detection (Priority over Primes)
	if strings.Contains(upperPrompt, "FEATURE_LIST.JSON") || strings.Contains(upperPrompt, "FEATURES") {
		return `Here is the feature list in JSON format.

` + "```json" + `
{
  "project_name": "recac-e2e",
  "features": [
    {
      "id": "1",
      "description": "Calculate prime numbers",
      "category": "core",
      "priority": "high",
      "status": "pending"
    }
  ]
}
` + "```", nil
	}

	// Mock Logic for Smoke Tests
	if strings.Contains(upperPrompt, "PRIMES") || strings.Contains(upperPrompt, "PRIME NUMBERS") {
		return `I will create a python script to calculate prime numbers.

` + "```bash" + `
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

if __name__ == "__main__":
    import sys
    n = int(sys.argv[1]) if len(sys.argv) > 1 else 10
    print([x for x in range(n) if is_prime(x)])
EOF

python3 primes.py 20

git add primes.py
git commit -m "feat: add primes.py" || echo "Nothing to commit"

if command -v agent-bridge >/dev/null 2>&1; then
    agent-bridge signal COMPLETED true
fi
` + "```", nil
	}

	if strings.Contains(upperPrompt, "SPEC") {
		return `I have analyzed the spec.

` + "```bash" + `
if command -v agent-bridge >/dev/null 2>&1; then
    agent-bridge signal COMPLETED true
fi
` + "```", nil
	}

	if strings.Contains(upperPrompt, "NOTHING TO COMMIT") || strings.Contains(upperPrompt, "WORKING TREE CLEAN") {
		return `It seems there is nothing to commit. I will mark the task as completed.

` + "```bash" + `
if command -v agent-bridge >/dev/null 2>&1; then
    agent-bridge signal COMPLETED true
fi
` + "```", nil
	}

	// Default response
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))

	// Ensure valid bash block for tests that expect it
	response += "\n\n```bash\n# No-op command to satisfy runner\necho 'Mock agent received command'\nif command -v agent-bridge >/dev/null 2>&1; then\n    agent-bridge signal COMPLETED true\nfi\n```"

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
