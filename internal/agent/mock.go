package agent

import (
	"context"
	"fmt"
	"regexp"
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

	// 1. Planning Phase Detection
	// The Orchestrator sends a prompt with "Technical Program Manager" or similar when asking for a plan.
	// We need to return a JSON array of tickets.
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "Create a plan") {
		if strings.Contains(prompt, "ID:[PRIMES]") {
			// Return a single ticket for the Prime Number Task
			// Note: We include the Repo URL from the prompt if possible, or a placeholder.
			// The JiraPoller might need it.

			// Extract Repo URL from prompt if present to be helpful
			repoURL := "https://github.com/example/repo"
			re := regexp.MustCompile(`(?i)Repo: (https?://\S+)`)
			matches := re.FindStringSubmatch(prompt)
			if len(matches) > 1 {
				repoURL = matches[1]
			}

			return fmt.Sprintf(`[
  {
    "title": "Implement Prime Number Script",
    "description": "Implement primes.py to calculate primes < 10000. Output to primes.json. Repo: %s",
    "type": "Task",
    "id": "PRIMES"
  }
]`, repoURL), nil
		}

		// Default Plan
		return `[{"title": "Mock Task", "description": "A mock task description", "type": "Task"}]`, nil
	}

	// 2. Coding Phase Detection
	// If the prompt contains the specific task ID, we return the implementation.
	if strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "Implement Prime Number Script") {
		// Return a response that satisfies the 'prime-python' scenario
		response := `Sure! Here is the python script to calculate primes:

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [n for n in range(10000) if is_prime(n)]

with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py
git add primes.json primes.py
git commit -m "Add primes script"
` + "```" + `

Task complete.
`
		return response, nil
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
