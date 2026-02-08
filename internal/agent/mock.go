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
// It includes heuristics for E2E scenarios like 'prime-python'
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristic for Ticket Planning (Scenario Generation)
	// The prompt from cmd/recac/jira.go typically contains "spec" and implies generating tickets.
	// We need to distinguish between "Plan tickets for PRIMES" and "Implement PRIMES".
	if strings.Contains(prompt, "tickets") || strings.Contains(prompt, "Story") || strings.Contains(prompt, "Epic") {
		if strings.Contains(prompt, "[PRIMES]") {
			return m.generatePrimesTickets(), nil
		}
	}

	// Heuristic for 'prime-python' scenario Implementation (Coding Agent)
	// If the prompt asks for implementation or we are in the initializer loop looking for PRIMES
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "[PRIMES]") {
		return m.generatePrimesImplementation(), nil
	}

	// Return a generic mock response
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

func (m *MockAgent) generatePrimesImplementation() string {
	return `I will create the prime number script as requested.

` + "```bash" + `
# 1. Initialize DB with the feature to satisfy the loop
cat << 'EOF' > features.json
[
  {
    "id": "PRIMES",
    "category": "Task",
    "priority": "MVP",
    "description": "Implement primes.py",
    "status": "in_progress",
    "passes": false,
    "dependencies": {
        "depends_on_ids": [],
        "exclusive_write_paths": [],
        "read_only_paths": []
    }
  }
]
EOF
agent-bridge import --file features.json

# 2. Implement the script
cat << 'EOF' > primes.py
import json

def get_primes(n):
    primes = []
    for i in range(2, n):
        is_prime = True
        for j in range(2, int(i**0.5) + 1):
            if i % j == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(i)
    return primes

primes = get_primes(10000)
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# 3. Commit and Push
python3 primes.py
git add primes.py primes.json
git commit -m "Add primes script"
git push

# 4. Mark done to stop the agent loop
agent-bridge feature set --id PRIMES --status done --passes true
` + "```" + `
`
}

func (m *MockAgent) generatePrimesTickets() string {
	return `
` + "```json" + `
[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.",
    "type": "Task",
    "children": []
  }
]
` + "```" + `
`
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
