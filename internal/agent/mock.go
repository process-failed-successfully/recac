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

	// 0. Role-Specific Logic (QA & Manager)
	// These specific roles are detected by the header in the prompt.
	if strings.Contains(prompt, "YOUR ROLE - QA AGENT") {
		return `I have run the tests and they passed.

` + "```bash" + `
echo "Running tests..."
# Simulate passing tests
agent-bridge signal QA_PASSED true
` + "```" + `
`, nil
	}

	if strings.Contains(prompt, "YOUR ROLE - PROJECT MANAGER") {
		return `I have reviewed the project and it meets all requirements.

` + "```bash" + `
echo "Project Approved"
agent-bridge signal PROJECT_SIGNED_OFF true
` + "```" + `
`, nil
	}

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
      "id": "req-primes-py-exists",
      "category": "functional",
      "priority": "critical",
      "description": "primes.py exists",
      "status": "todo",
      "passes": false,
      "steps": [],
      "dependencies": {
          "depends_on_ids": [],
          "exclusive_write_paths": [],
          "read_only_paths": []
      }
    },
    {
      "id": "req-primes-json-contains-correct-p",
      "category": "functional",
      "priority": "critical",
      "description": "primes.json contains correct primes",
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

cat feature_list.json | agent-bridge import
` + "```" + `
`, nil
	}

	// 3. Implementation for 'prime-python' scenario
	// The prompt will typically contain the ticket description or "primes.py" instructions.
	// We use a "greedy" match here: if it talks about the primes task AND it's NOT the ticket generation prompt (checked above),
	// assume it's the coding task.
	// We check for keywords related to the task.
	lowerPrompt := strings.ToLower(prompt)
	// Broaden match: If it mentions primes/req-primes OR contains "feature_list.json" (Coding Agent template) without being initializer
	isPrimesTask := strings.Contains(lowerPrompt, "primes") ||
		strings.Contains(lowerPrompt, "req-primes") ||
		(strings.Contains(lowerPrompt, "feature_list.json") && !strings.Contains(lowerPrompt, "initialize"))

	if isPrimesTask {
		// Smart Check: If the prompt indicates that we already tried to commit and it was empty,
		// it means the files are already there and correct. We should mark the task as done.
		if strings.Contains(lowerPrompt, "nothing to commit") || strings.Contains(lowerPrompt, "working tree clean") {
			return `The task seems to be completed. I will mark it as done.

` + "```bash" + `
agent-bridge feature list --json | jq -r '.features[].id' | xargs -I {} agent-bridge feature set {} --status done --passes true
agent-bridge signal COMPLETED true
` + "```" + `
`, nil
		}

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

	// Debugging: Log the prompt to help diagnose why heuristics failed
	// This will show up in CI logs (stdout)
	if len(prompt) < 1000 {
		fmt.Printf("[MockAgent] Fallback Triggered. Prompt: %s\n", prompt)
	} else {
		fmt.Printf("[MockAgent] Fallback Triggered. Prompt (truncated): %s...\n", prompt[:1000])
	}

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
