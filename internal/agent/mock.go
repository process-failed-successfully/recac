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

	// Smart Mock Logic for Smoke Tests

	// 1. Ticket Generation for 'prime-python' scenario
	// Matches strict requirements from AppSpec (ID:[PRIMES] header + Task instruction)
	// We specifically look for the instruction to CREATE the ticket.
	if strings.Contains(prompt, "ID:[PRIMES]") && (strings.Contains(prompt, "Type: Task") || strings.Contains(prompt, "create exactly ONE ticket")) {
		return `[
  {
    "title": "ID:[PRIMES] Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.\n\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Task",
    "acceptance_criteria": [
      "primes.py exists",
      "primes.json contains correct primes"
    ],
	"children": []
  }
]`, nil
	}

	// 2. Initialization for 'prime-python' scenario
	// Detects "agent-bridge import" or "Feature List" + "initialize" to generate the feature list.
	// This must come BEFORE implementation check if there's overlap in keywords, or we ensure implementation check is specific enough.
	if strings.Contains(prompt, "agent-bridge import") || (strings.Contains(prompt, "Feature List") && strings.Contains(prompt, "initialize")) {
		return `I will create the feature list and import it.

` + "```bash" + `
cat << 'EOF' > feature_list.json
{
  "project_name": "primes-project",
  "features": [
    {
      "id": "req-primes",
      "category": "core",
      "priority": "MVP",
      "description": "Calculate primes",
      "status": "todo",
      "passes": false,
      "steps": [],
      "dependencies": {
          "depends_on_ids": [],
          "exclusive_write_paths": [],
          "read_only_paths": []
      }
    }
  ]
}
EOF

agent-bridge import --file feature_list.json
` + "```" + `
`, nil
	}

	// 3. Completion Check (Must be before Implementation Check)
	// If the previous command output indicates nothing to commit (clean working tree), we are done.
	// This prevents infinite loops in smoke tests where the agent keeps trying to commit.
	promptLower := strings.ToLower(prompt)
	if strings.Contains(promptLower, "nothing to commit") || strings.Contains(promptLower, "working tree clean") {
		return `It seems there are no more changes to commit. The task is complete.

` + "```bash" + `
if command -v agent-bridge >/dev/null 2>&1; then
    agent-bridge signal COMPLETED true
else
    echo "agent-bridge not found, cannot signal completion."
fi
` + "```" + `
`, nil
	}

	// 4. Implementation for 'prime-python' scenario
	// The prompt will typically contain the ticket description or "primes.py" instructions.
	// We use a "greedy" match here: if it talks about the primes task AND it's NOT the ticket generation prompt (checked above),
	// assume it's the coding task.
	// We check for keywords related to the task.
	isPrimesTask := strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "primes.json") || strings.Contains(prompt, "req-primes")

	if isPrimesTask {
		return `I will create the primes.py script and generate the JSON file as requested.

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

primes = [x for x in range(10000) if is_prime(x)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Configure git for commit
git config user.email "mock@example.com"
git config user.name "Mock Agent"

python3 primes.py
git add primes.py primes.json
git commit -m "Add primes.py and primes.json" || echo "Nothing to commit"
` + "```" + `
`, nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf(`%s:

I received your prompt (%d characters). In mock mode, I would process this request and provide a response.

`+"```bash"+`
# no-op
echo "Acknowledged"
`+"```"+`
`, m.responsePrefix, len(prompt))
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
