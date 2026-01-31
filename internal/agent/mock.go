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

	// Heuristic for "prime-python" scenario
	// 1. Planning Phase: Prompt contains ID:[PRIMES] (from scenario AppSpec) AND "Create a JSON object" (from prompt template)
	if strings.Contains(prompt, "ID:[PRIMES]") && strings.Contains(prompt, "Create a JSON object") {
		return "```json\n" +
			`{
  "tickets": [
    {
      "id": "PRIMES",
      "type": "Task",
      "title": "Create Prime Number Script",
      "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'."
    }
  ]
}` + "\n```", nil
	}

	// 2. Execution Phase: Prompt contains "primes.py" but NOT "ID:[PRIMES]" (which is in the spec, not the task instruction usually)
	// Actually, task instructions might contain ID if passed from Jira.
	// A safer heuristic for execution is if it asks to "Implement" or contains code-like instructions without "Create a JSON object".
	if strings.Contains(prompt, "primes.py") && !strings.Contains(prompt, "Create a JSON object") {
		// Return a bash script to do the work
		return "```bash\n" +
			`# Create the python script
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [p for p in range(2, 10000) if is_prime(p)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run it
python3 primes.py

# Commit and Push
# Note: In real agent, git setup is done.
git add primes.py primes.json
git commit -m "Implement primes calculation"
git push
` + "\n```", nil
	}

	// Default Mock Response
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...\n\n```bash\n# no-op to satisfy circuit breaker\necho 'mock agent alive'\n```",
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
