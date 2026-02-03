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

	// Detect Ticket Generation Prompt
	if strings.Contains(prompt, "Technical Program Manager") {
		// Return valid JSON ticket list to satisfy parser
		return `[
  {
    "id": "MOCK-1",
    "title": "Implement Core Feature",
    "description": "Repo: https://github.com/process-failed-successfully/recac-jira-e2e\n\nImplement the primary functionality requested in the spec.",
    "type": "Task",
    "status": "To Do",
    "priority": "High",
    "dependencies": [],
    "acceptance_criteria": [
      "Create primes.py"
    ]
  }
]`, nil
	}

	// Detect Implementation Prompt (primes.py)
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "Implement Core Feature") {
		// Return bash script to implement the feature
		return `I will implement the primes.py script as requested.

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

primes = [i for i in range(10000) if is_prime(i)]
print(f"Found {len(primes)} primes")

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run the script to generate the json
python3 primes.py

# Commit the changes
git add primes.py primes.json
git commit -m "Implement primes.py"
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
