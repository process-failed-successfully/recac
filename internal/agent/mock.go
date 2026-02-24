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

	// Smart Mocking for Smoke Tests
	// If the prompt looks like the Prime Python spec, return a valid JSON plan AND bash execution
	// The Orchestrator REQUIRES a bash block to execute commands. JSON-only responses cause a NO-OP loop.
	if strings.Contains(prompt, "ID:[PRIMES] Prime Number Script") {
		return `
I will create the prime number script as requested.

[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Create a python script named 'primes.py'. It MUST be python.\nIt must calculate all prime numbers less than 10,000 and output to a file named 'primes.json'.",
    "type": "Task",
    "children": []
  }
]

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

primes = [x for x in range(10000) if is_prime(x)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run the script to generate the json
python3 primes.py

# Commit the files
git add primes.py primes.json
git commit -m "Add primes.py and generated json"
` + "```" + `
`, nil
	}

	if strings.Contains(prompt, "Create a python script named 'primes.py'") {
		return `I will implement the prime number script as requested.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [i for i in range(10000) if is_prime(i)]

with open('primes.json', 'w') as f:
    json.dump({'primes': primes}, f)
EOF

python3 primes.py
git add primes.py primes.json
git commit -m "Add primes implementation"
git push

# Mark project as signed off to trigger early exit in smoke tests
recac signal set PROJECT_SIGNED_OFF true
` + "```" + `
`, nil
	}

	if strings.Contains(prompt, "Create a guided tour of this codebase") {
		return `[
  {
    "title": "Project Overview",
    "filepath": "README.md",
    "description": "# Project Overview\n\nWelcome to **recac**! This is a distributed autonomous coding framework.\n\nKey features:\n- **Orchestrator**: Manages tasks and agents.\n- **Agent**: The core autonomous coding logic.\n- **CLI**: The command-line interface you are using now."
  },
  {
    "title": "Entry Point",
    "filepath": "cmd/recac/main.go",
    "description": "This is the main entry point for the CLI. It uses ` + "`cobra`" + ` to handle commands."
  },
  {
    "title": "Core Logic",
    "filepath": "internal/runner/runner.go",
    "description": "The ` + "`runner`" + ` package contains the core loop for the autonomous agent. It manages the session, state, and execution of actions."
  }
]`, nil
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
