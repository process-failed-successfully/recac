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

	// 1. Ticket Generation / Planning (TPM)
	// Triggers when "Technical Program Manager" role is detected
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "generate-from-spec") {
		return `[
  {
    "id": "PRIMES",
    "type": "Epic",
    "title": "Implement Prime Number Generator",
    "description": "Create a python script to generate prime numbers. Repo: https://github.com/process-failed-successfully/recac",
    "dependencies": []
  },
  {
    "id": "req-primes-json-contains-correct-p",
    "type": "Task",
    "title": "Implement primes.py",
    "description": "Implement the prime number generation logic.",
    "parent_id": "PRIMES",
    "dependencies": []
  }
]`, nil
	}

	// 2. Initialization (Feature List)
	// Triggers when "Initializer" role or JSON format request is detected
	if strings.Contains(prompt, "Initializer") || strings.Contains(prompt, "JSON format") {
		return `Here is the feature list:
` + "```bash" + `
cat <<EOF > feature_list.json
{
  "features": [
    {
      "id": "req-primes-json-contains-correct-p",
      "name": "Implement primes.py",
      "description": "Implement the prime number generation logic.",
      "status": "pending",
      "dependencies": {}
    }
  ]
}
EOF
cat feature_list.json | agent-bridge import || echo "Warning: agent-bridge import failed"
` + "```", nil
	}

	// 3. Coding Phase (Implementation)
	// Triggers when specific files or tasks are mentioned
	if strings.Contains(prompt, "primes.py") {
		return `I'll implement the primes script:
` + "```bash" + `
cat <<EOF > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [p for p in range(1, 101) if is_prime(p)]
print(json.dumps(primes))
EOF

# Mark as done
if command -v agent-bridge > /dev/null; then
	agent-bridge feature set req-primes-json-contains-correct-p status=completed || echo "Warning: failed to update feature status"
fi
` + "```", nil
	}

	// 4. Completion / Loop Prevention
	lowerPrompt := strings.ToLower(prompt)
	if strings.Contains(lowerPrompt, "nothing to commit") || strings.Contains(lowerPrompt, "working tree clean") {
		return `Nothing to commit, marking as done:
` + "```bash" + `
if command -v agent-bridge > /dev/null; then
    agent-bridge signal COMPLETED true
else
    echo "agent-bridge not found"
fi
` + "```", nil
	}

	// Default fallback
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
