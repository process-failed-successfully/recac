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

	// Heuristics for "prime-python" scenario

	// 1. Ticket Generation Phase (TPM Agent / CLI)
	// Triggered by "recac jira generate-from-spec"
	// Prompt contains "Technical Program Manager" and "ID:[PRIMES]" (from spec)
	if strings.Contains(prompt, "Technical Program Manager") && strings.Contains(prompt, "ID:[PRIMES]") {
		// Expects pure JSON list of tickets
		return `
[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates primes < 10000. Output to 'primes.json'. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Task",
    "children": []
  }
]`, nil
	}

	// 2. Planning Phase (Initializer Agent / Loop)
	// Triggered by Orchestrator at start of session
	// Prompt contains "Create feature_list.json" (from spec)
	if strings.Contains(prompt, "Create feature_list.json") {
		return `I will generate the feature list.

` + "```bash" + `
cat << 'EOF' | agent-bridge import
{
  "project_name": "Prime Number Script",
  "features": [
    {
      "id": "PRIMES",
      "category": "functional",
      "priority": "MVP",
      "description": "Implement a python script named 'primes.py' that calculates primes < 10000. Output to 'primes.json'.",
      "status": "pending",
      "passes": false,
      "steps": [
        "Run python3 primes.py",
        "Check if primes.json exists",
        "Validate JSON structure"
      ],
      "dependencies": {
        "depends_on_ids": [],
        "exclusive_write_paths": ["primes.py", "primes.json"],
        "read_only_paths": []
      }
    }
  ]
}
EOF
` + "```" + `
`, nil
	}

	// 3. Execution Phase (Coding Agent)
	// The prompt usually asks to "Implement ... primes.py" and output to "primes.json"
	// It comes from the "PRIMES" feature description we just created.
	if strings.Contains(prompt, "primes.py") && strings.Contains(prompt, "primes.json") {
		return `I will create the primes script.

` + "```bash" + `
# Create primes.py
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [n for n in range(10000) if is_prime(n)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run the script
python3 primes.py

# Verify output
cat primes.json

# Commit results
git add primes.py primes.json
git commit -m "Add primes script and output"

# Mark feature as done
agent-bridge feature set PRIMES --status done --passes true
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
