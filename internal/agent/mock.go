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

	// CI Smoke Test Support: prime-python scenario
	if strings.Contains(prompt, "ID:[PRIMES]") {
		// Differentiate between Planner (JSON Request) and Coding Agent (Bash Script Request)

		// Planner Prompt Detection (Broader Check)
		if strings.Contains(prompt, "feature list") || strings.Contains(prompt, "JSON object") {
			return `{
  "project_name": "Prime Number Script",
  "features": [
    {
      "id": "TASK-1",
      "category": "functional",
      "description": "Implement prime number script",
      "status": "pending",
      "steps": [
        "Create primes.py",
        "Implement prime calculation logic",
        "Run script",
        "Verify primes.json exists"
      ],
      "dependencies": {
        "depends_on_ids": [],
        "exclusive_write_paths": [],
        "read_only_paths": []
      }
    }
  ]
}`, nil
		}

		// Fallback: Assume Coding Agent request (or any request asking for execution/script)
		return `I will create the prime number script as requested.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [i for i in range(2, 10000) if is_prime(i)]

with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py
git add primes.py primes.json
git commit -m "Add primes.py and primes.json"
` + "```" + `
`, nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
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
