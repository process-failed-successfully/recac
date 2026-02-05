package agent

import (
	"context"
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

	// Heuristic: Detect Ticket Generation Request (TPM Agent)
	// We check for the explicit role definition to avoid false positives in prompt history
	// Note: prompt template might say "You are an expert Technical Program Manager"
	if strings.Contains(prompt, "Technical Program Manager") {
		// Specific handling for the [PRIMES] scenario (Smoke Test) which strictly requires a single Task
		if strings.Contains(prompt, "[PRIMES]") {
			return `[
  {
    "id": "PRIMES",
    "title": "ID:[PRIMES] Prime Number Script",
    "description": "Repo: https://github.com/process-failed-successfully/recac-jira-e2e\nImplement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.",
    "type": "Task",
    "acceptance_criteria": ["Implement primes.py script", "Verify primes.json output"]
  }
]`, nil
		}

		// Default fallback for other scenarios
		return `[
  {
    "id": "PRIMES",
    "title": "ID:[PRIMES] Epic: Implement Core Features",
    "description": "Repo: https://github.com/process-failed-successfully/recac-jira-e2e\nImplement the core functionality described in the spec.",
    "type": "Epic",
    "children": [
      {
        "id": "PRIMES-1",
        "title": "ID:[PRIMES-1] Story: Implement Primary Logic",
        "description": "Repo: https://github.com/process-failed-successfully/recac-jira-e2e\nDevelop the main script/application logic.",
        "type": "Story",
        "acceptance_criteria": ["Logic is implemented", "Tests pass"]
      }
    ]
  }
]`, nil
	}

	// 1. Initializer Role
	if strings.Contains(strings.ToLower(prompt), "initializer agent") {
		return `I will initialize the project features.
` + "```bash" + `
echo '{"features": [{"id": "req-1", "description": "impl", "status": "todo"}]}' > feature_list.json
cat feature_list.json | agent-bridge import
` + "```", nil
	}

	// 2. QA Role
	if strings.Contains(prompt, "YOUR ROLE - QA AGENT") {
		return `QA Checks Passed.
` + "```bash" + `
agent-bridge signal QA_PASSED true
` + "```", nil
	}

	// 3. Manager Role
	if strings.Contains(prompt, "YOUR ROLE - PROJECT MANAGER") {
		return `Project Approved.
` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true
` + "```", nil
	}

	// 4. Completion Check (Prevent Loop)
	if strings.Contains(strings.ToLower(prompt), "nothing to commit") {
		return `No changes to commit. Marking features as done to proceed.
` + "```bash" + `
agent-bridge feature list --json | jq -r '.features[].id' | xargs -I {} agent-bridge feature set {} --status done --passes true
agent-bridge signal COMPLETED true
` + "```", nil
	}

	// 5. Default Coding Agent (Smoke Test Logic)
	// If we are in a coding loop (default), generate code and update features.

	// Specific handling for primes.py (Smoke Test)
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "[PRIMES]") {
		return `I will implement the primes script.
` + "```bash" + `
set -e
cat << 'EOF' > primes.py
import json
import sys

primes = []
for n in range(2, 10000):
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            break
    else:
        primes.append(n)

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)

print(f"Generated {len(primes)} primes to primes.json")
EOF

python3 primes.py
if [ ! -f primes.json ]; then
    echo "Error: primes.json was not created"
    exit 1
fi

git add primes.py primes.json
git commit --author="Recac Agent <agent@recac.io>" -m "Add primes script"
git push
agent-bridge feature list --json | jq -r '.features[].id' | xargs -I {} agent-bridge feature set {} --status done --passes true
agent-bridge signal COMPLETED true
` + "```", nil
	}

	// Fallback generic approach
	return `I will implement the requested logic and update feature status.
` + "```bash" + `
# Create a dummy implementation file to satisfy requirements
echo "def is_prime(n): return n > 1" > primes.py

# Mark all features as done and passing
agent-bridge feature list --json | jq -r '.features[].id' | xargs -I {} agent-bridge feature set {} --status done --passes true
` + "```", nil
}

// SendStream implements the Agent interface
func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := m.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(resp)
	}
	return resp, err
}

