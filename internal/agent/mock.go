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

	// Detect TPM Agent prompts (Ticket Generation) and return JSON
	if strings.Contains(prompt, "Technical Program Manager") && strings.Contains(prompt, "Application Specification") {
		// Specific response for "prime-python" scenario
		if strings.Contains(prompt, "ID:[PRIMES]") {
			return `[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.\n\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Task",
    "children": []
  }
]`, nil
		}

		// Generic JSON response for other TPM prompts
		return `[
  {
    "title": "ID:[MOCK-1] Mock Epic",
    "description": "Mock Epic Description.\nRepo: https://github.com/example/repo",
    "type": "Epic",
    "children": []
  }
]`, nil
	}

	// Detect Initializer Agent prompts (creates feature_list.json)
	if strings.Contains(prompt, "INITIALIZER AGENT") {
		// Create a feature list for the primes scenario
		return "```bash\n" + `cat << 'EOF' > feature_list.json
[
  {
    "id": "req-primes-py-exists",
    "description": "Create primes.py script",
    "status": "pending",
    "verification_cmd": "test -f primes.py"
  },
  {
    "id": "req-primes-json-contains-correct-primes",
    "description": "Run primes.py and verify primes.json output",
    "status": "pending",
    "verification_cmd": "python3 primes.py && grep -q 'primes' primes.json"
  }
]
EOF
` + "\n```", nil
	}

	// Detect Implementation Prompts (Coding Agent)
	if strings.Contains(prompt, "primes.py") {
		// Return a script that implements the solution
		return "```bash\n" + `cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [n for n in range(10000) if is_prime(n)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py
git add primes.py primes.json
git commit -m "Add primes script" || echo "Nothing to commit"
agent-bridge feature set req-primes-py-exists --status done --passes true
agent-bridge feature set req-primes-json-contains-correct-primes --status done --passes true
` + "\n```", nil
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
