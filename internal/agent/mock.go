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

	// Heuristics for E2E scenarios
	lowerPrompt := strings.ToLower(prompt)

	// 1. Initializer Agent
	if strings.Contains(lowerPrompt, "initializer agent") {
		return `I will initialize the project with the required feature list.

` + "```bash" + `
cat << 'EOF' | agent-bridge import
{
  "project_name": "Mock Project",
  "features": [
    {
      "id": "PRIMES",
      "category": "functional",
      "priority": "MVP",
      "description": "Calculate primes less than 10000",
      "status": "pending",
      "steps": ["Run primes.py", "Check primes.json"],
      "passes": false,
      "dependencies": {
        "depends_on_ids": [],
        "exclusive_write_paths": [],
        "read_only_paths": []
      }
    }
  ]
}
EOF

# Create init.sh as requested
echo "#!/bin/bash" > init.sh
chmod +x init.sh

# Initial commit
if [ ! -d ".git" ]; then
	git init
	git add .
	git commit -m "Initial commit"
fi
` + "```" + `
`, nil
	}

	// 2. Ticket Generation (Planning Phase)
	// Must come BEFORE coding phase because both prompts mention "primes.py"
	if strings.Contains(lowerPrompt, "create exactly one ticket") {
		// Return pure JSON as expected by recac jira parser
		// Note: The parser is strict about JSON, so we output ONLY JSON
		return `[
  {
    "id": "PRIMES",
    "title": "Create Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.",
    "type": "Task",
    "status": "Open",
    "priority": "High"
  }
]`, nil
	}

	// 3. Coding Agent (Primes Scenario Implementation)
	if strings.Contains(lowerPrompt, "primes.py") || strings.Contains(prompt, "[PRIMES]") {
		return `I will implement the primes.py script as requested.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(10000) if is_prime(x)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py
git add primes.py primes.json
git commit -m "Add primes.py and primes.json"
git push origin HEAD

agent-bridge feature set PRIMES --status completed --passes true
` + "```" + `
`, nil
	}

	// 4. QA Agent / Manager
	if strings.Contains(lowerPrompt, "qa agent") || strings.Contains(lowerPrompt, "approve or reject") {
		return `I have reviewed the changes and they look correct.

` + "```bash" + `
agent-bridge signal QA_PASSED true --privileged
` + "```" + `
`, nil
	}

	// Default response (echo)
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
