package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a smart mock agent for testing and mock mode
// It uses heuristics to return useful responses for known scenarios (like E2E tests)
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
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// 1. Planner Agent (JSON) Heuristic
	// The planner prompt always starts with "## ROLE: Lead Software Architect"
	if strings.Contains(prompt, "ROLE: Lead Software Architect") || strings.Contains(prompt, "Lead Software Architect") {
		return `{
  "project_name": "primes_project",
  "features": [
    {
      "id": "PRIMES",
      "category": "functional",
      "description": "Script calculates primes correctly",
      "status": "pending",
      "steps": [
        "Create primes.py",
        "Run script",
        "Verify primes.json output"
      ],
      "dependencies": {
        "depends_on_ids": [],
        "exclusive_write_paths": [],
        "read_only_paths": []
      }
    }
  ]
}`, nil
	}

	// 2. Initializer Agent Heuristic
	// The initializer prompt contains "ROLE - INITIALIZER AGENT"
	if strings.Contains(strings.ToUpper(prompt), "ROLE - INITIALIZER AGENT") {
		// Return commands to initialize the project DB/Structure
		return `I will initialize the project.

` + "```bash" + `
cat <<EOF | agent-bridge import
{
  "project_name": "primes_project",
  "features": [
    {
      "id": "PRIMES",
      "category": "functional",
      "description": "Script calculates primes correctly",
      "status": "pending",
      "steps": [
        "Create primes.py",
        "Run script",
        "Verify primes.json output"
      ],
      "dependencies": {
        "depends_on_ids": [],
        "exclusive_write_paths": [],
        "read_only_paths": []
      }
    }
  ]
}
EOF
` + "```" + `
`, nil
	}

	// 3a. Technical Program Manager (Ticket Generation) Heuristic
	// Used by 'jira generate-from-spec'. Expects array of tickets.
	// Check for "application specification" or "decompose" in prompt.
	if strings.Contains(prompt, "Technical Program Manager") && (strings.Contains(prompt, "application specification") || strings.Contains(prompt, "decompose")) {
		return `[
  {
    "title": "ID:[PRIMES] Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Task",
    "children": []
  }
]`, nil
	}

	// 3b. Technical Program Manager (Status) Heuristic
	// Fallback for other TPM prompts (status updates).
	if strings.Contains(prompt, "Technical Program Manager") {
		return `{
  "project_status": "on_track",
  "summary": "Project is proceeding according to plan.",
  "risk_assessment": "low",
  "next_steps": ["Continue implementation"]
}`, nil
	}

	// 4. Project Manager / QA Heuristic
	// If the prompt asks for status or QA
	if strings.Contains(prompt, "ROLE: Project Manager") || strings.Contains(prompt, "QA") {
		// If prompt contains "failures" or "failed features", tell them to fix it (simulated by saying done for now to break loops if persistent)
		// Actually, if we are in a test loop, we want to finish.

		// If we see "pending" OR if we see "passes: true" / "done" without explicit failure signal
		// We should just sign off.

		return `The feature looks complete.

` + "```bash" + `
agent-bridge feature set PRIMES --status done --passes true
agent-bridge signal create QA_PASSED true
agent-bridge signal create PROJECT_SIGNED_OFF
` + "```" + `
`, nil
	}

	// 5. Coding Agent - Prime Python Scenario
	// Only trigger this if we are NOT the planner/TPM (handled above)
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "[PRIMES]") {

		// Guard: If we've already implemented it, don't loop forever.
		if strings.Contains(prompt, "primes.json") && strings.Contains(prompt, "implemented") {
			return "Task PRIMES is already implemented. No further actions needed.", nil
		}

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

primes = [x for x in range(10000) if is_prime(x)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF
` + "```" + `

Now I will run the script and commit the results.

` + "```bash" + `
python3 primes.py || python primes.py
git add -f primes.json
git add primes.py
git commit -m "Add primes script and output" || echo "No changes to commit"
` + "```" + `
`, nil
	}

	// 6. Default/Fallback
	// Log the prompt prefix to help debug
	fmt.Printf("DEBUG: MockAgent Prompt: %s\n", truncateString(prompt, 50))

	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response.",
		m.responsePrefix, len(prompt))

	// Add a safe command to prevent "NO-OP LOOP" errors in some runners
	response += "\n\n```bash\necho \"Mock Agent Default Response\"\n```"

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
