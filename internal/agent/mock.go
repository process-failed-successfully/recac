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

	// --- Heuristics for E2E Scenarios (e.g. Smoke Test) ---

	// 1. Initializer Role: Detects "YOUR ROLE - INITIALIZER AGENT" or "Feature list not found"
	// Returns a feature list via agent-bridge import
	if (strings.Contains(prompt, "YOUR ROLE - INITIALIZER AGENT") || strings.Contains(prompt, "Feature list not found")) &&
		(strings.Contains(strings.ToLower(prompt), "prime") || strings.Contains(prompt, "[PRIMES]")) {
		return `Here is the plan for the Prime Number Script:

` + "```bash" + `
cat << 'EOF' | agent-bridge import
{
  "project_name": "Primes Python",
  "features": [
    {
      "id": "req-primes-implementation",
      "category": "functional",
      "priority": "MVP",
      "description": "Script calculates primes correctly and outputs json",
      "status": "pending",
      "steps": [
        "Run python primes.py",
        "Check primes.json exists",
        "Verify 1229 primes"
      ],
      "dependencies": {
        "exclusive_write_paths": ["primes.py", "primes.json"],
        "read_only_paths": []
      }
    }
  ]
}
EOF
` + "```", nil
	}

	// 2. Coding Agent: Detects "YOUR ROLE - CODING AGENT" and "Prime"
	// Implements the script and marks the feature as done
	if (strings.Contains(prompt, "YOUR ROLE - CODING AGENT") || strings.Contains(prompt, "role selected: Agent")) &&
		(strings.Contains(strings.ToLower(prompt), "prime") || strings.Contains(prompt, "req-primes-implementation")) {
		return `I will implement the primes script as requested.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [x for x in range(10000) if is_prime(x)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run it to generate the json
python3 primes.py

# Commit
git add -f primes.json primes.py
git commit -m "Add primes script" || echo "Nothing to commit"

# Mark as done
agent-bridge feature set req-primes-implementation --status done --passes true
` + "```", nil
	}

	// 3. QA Agent: Mark QA as passed
	if strings.Contains(prompt, "YOUR ROLE - QA AGENT") {
		return `QA checks passed.

` + "```bash" + `
agent-bridge signal QA_PASSED true
` + "```", nil
	}

	// 4. Manager Agent: Approve
	if strings.Contains(prompt, "YOUR ROLE - MANAGER AGENT") {
		return `Approved.

` + "```bash" + `
# Manager approval is handled by the runner when no blockers are found.
echo "Manager Approved"
` + "```", nil
	}

	// 4. TPM / Planning Role (Ticket Generation)
	// This is critical for 'recac jira generate-from-spec'
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "Ticket Generation") {
		// Detect Primes Scenario (used in smoke test)
		if strings.Contains(strings.ToLower(prompt), "prime") || strings.Contains(prompt, "[PRIMES]") {
			return `Here is the ticket plan:

` + "```json" + `
[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'. The JSON format must have a single key 'primes' containing the list of integers. IMPORTANT: You MUST use a bash block to create the file. Commit 'primes.json' IMMEDIATELY after creating/running the script.",
    "type": "Task",
    "children": []
  }
]
` + "```", nil
		}

		// Default Mock Plan (for other tests)
		return `Here is the ticket plan:

` + "```json" + `
[
  {
    "title": "ID:[SYSTEM] Mock System Architecture",
    "description": "Mock system implementation.",
    "type": "Epic",
    "children": [
      {
        "title": "ID:[SERVICE] Mock Service",
        "description": "Mock service description.",
        "type": "Story",
        "children": []
      }
    ]
  }
]
` + "```", nil
	}

	// --- Fallback ---

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	// We include a dummy shell command to prevent the "NO-OP LOOP" circuit breaker from tripping in CI.
	response := fmt.Sprintf(`%s:

I received your prompt (%d characters).

Here is a no-op command to satisfy the runner loop:

`+"```bash"+`
echo "Mock Agent: processing request..."
`+"```"+`

Prompt preview: %s...`,
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
