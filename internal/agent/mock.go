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

	// Heuristic for "prime-python" scenario planning phase.
	// The planner prompt includes the AppSpec.
	// We check for the specific ID used in the spec.
	if strings.Contains(prompt, "ID:[PRIMES]") && (strings.Contains(prompt, "AppSpec") || strings.Contains(prompt, "Specification")) {
		// Return a JSON ARRAY of tickets, as expected by cmd/recac/jira.go
		return `[
    {
      "title": "ID:[PRIMES] Create Prime Number Script",
      "description": "Create a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to 'primes.json'.",
      "type": "Task"
    }
  ]`, nil
	}

	// Heuristic for "prime-python" scenario execution phase.
	// We want to match the task execution prompt but NOT the planning prompt.
	// The planning prompt also contains "primes.py" and "Create", so we must exclude it.
	isPlanning := strings.Contains(prompt, "AppSpec") || strings.Contains(prompt, "Specification")
	if !isPlanning && (strings.Contains(prompt, "Task: [PRIMES]") || (strings.Contains(prompt, "primes.py") && strings.Contains(prompt, "Create"))) {
		return fmt.Sprintf(`
I will implement the prime number script.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n %% i == 0: return False
    return True

primes = [p for p in range(10000) if is_prime(p)]

with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py

# Add to git
git add primes.py primes.json
git commit -m "Add primes.py and primes.json"
` + "```" + `
`), nil
	}

	// Heuristic for Initializer Prompt (Feature List)
	// This handles the "no-op loop" where the agent is asked to initialize features but returns nothing useful.
	if strings.Contains(prompt, "agent-bridge import") || (strings.Contains(prompt, "Feature List") && strings.Contains(prompt, "initialize")) {
		// Return generic feature list for ANY project initialization prompt
		return `
I will initialize the project features.

` + "```bash" + `
cat << 'EOF' > feature_list.json
{
  "project_name": "prime-python",
  "features": [
    {
      "id": "1",
      "description": "Calculate prime numbers up to 10,000",
      "status": "pending",
      "file_paths": ["primes.py"],
      "test_paths": []
    }
  ]
}
EOF

# Import features to DB
agent-bridge import feature_list.json
` + "```" + `
`, nil
	}

	// Default Mock Response (Avoid empty response which trips No-Op circuit breaker)
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
