package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a smart mock agent for testing and mock mode
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

	// 1. Initializer Agent
	// Detect "Initializer" role or "git init" intent
	if strings.Contains(prompt, "Initializer") || strings.Contains(prompt, "INITIALIZER") {
		return m.initializerResponse(), nil
	}

	// 2. TPM Agent (Plan)
	// Detect "Technical Program Manager" or "TPM"
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") {
		if strings.Contains(prompt, "[PRIMES]") {
			return m.primesPlanResponse(), nil
		}
		// Default plan
		return m.defaultPlanResponse(), nil
	}

	// 3. QA Agent (Check before Coding Agent to prevent false positives with keywords)
	// STRICTER CHECK: Ensure we are actually assigned the QA role, not just mentioned
	if strings.Contains(prompt, "You are the QA Agent") || strings.Contains(prompt, "Your role is QA Agent") || strings.Contains(prompt, "ROLE - QA AGENT") {
		return m.qaAgentResponse(), nil
	}

	// 4. Coding Agent
	// Detect "Developer" or "Coding Agent", or explicit [PRIMES] tag (handles cases where system prompt is missing in mock mode)
	if strings.Contains(prompt, "Developer") || strings.Contains(prompt, "Coding Agent") || strings.Contains(prompt, "[PRIMES]") {
		lowerPrompt := strings.ToLower(prompt)
		// Detect "primes" intent
		if strings.Contains(lowerPrompt, "prime") || strings.Contains(prompt, "[PRIMES]") {
			return m.primesImplementationResponse(), nil
		}
	}

	// 5. Manager Agent
	// Detect "Manager" role (often used for final review/sign-off)
	// STRICTER CHECK: Ensure "Role" context or explicit Project Manager title
	if strings.Contains(prompt, "ROLE - PROJECT MANAGER") || strings.Contains(prompt, "You are the Project Manager") {
		return m.managerSignOffResponse(), nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	// We include a dummy command to prevent the "NO-OP LOOP" circuit breaker from tripping
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request.\n\nPrompt preview: %s...\n\n```bash\necho \"Mock Agent: Processing prompt...\"\n```",
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

func (m *MockAgent) initializerResponse() string {
	// Returns a script to initialize the repo and run agent-bridge import
	// Use standard markdown code block
	script := `#!/bin/bash
git init
git config user.email "bot@recac.com"
git config user.name "Recac Bot"
git add .
git commit --allow-empty -m "Initial commit"

cat <<EOF | agent-bridge import
{
  "project_name": "Mock Project",
  "features": [
    {
      "id": "PRIMES",
      "category": "Backend",
      "priority": "MVP",
      "description": "Create a script primes.py that calculates prime numbers up to 100 and saves them to primes.json. [PRIMES]",
      "status": "pending",
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
`
	return fmt.Sprintf("Here is the initialization script:\n\n```bash\n%s\n```", script)
}

func (m *MockAgent) primesPlanResponse() string {
	// Returns the JSON ticket list for the PRIMES scenario
	return `[
  {
    "id": "PRIMES",
    "title": "Calculate Primes",
    "description": "Create a script primes.py that calculates prime numbers up to 100 and saves them to primes.json. [PRIMES]",
    "type": "Task",
    "status": "Open",
    "assigned_to": "Developer"
  }
]`
}

func (m *MockAgent) defaultPlanResponse() string {
	return `[
  {
    "id": "TASK-1",
    "title": "Default Task",
    "description": "This is a default task from the mock agent.",
    "type": "Task",
    "status": "Open",
    "assigned_to": "Developer"
  }
]`
}

func (m *MockAgent) primesImplementationResponse() string {
	// Returns the implementation script for primes.py
	// Must use cat <<EOF for file creation and run python3
	return `Here is the implementation:

` + "```bash" + `
#!/bin/bash
cat <<EOF > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(1, 101) if is_prime(x)]
print(primes)

with open('primes.json', 'w') as f:
    json.dump(primes, f)
EOF

python3 primes.py
git add primes.py primes.json
git commit -m "Implement primes calculation" --author="Recac Bot <bot@recac.com>"
agent-bridge feature set PRIMES --status done --passes true
` + "```"
}

func (m *MockAgent) qaAgentResponse() string {
	return `## QA Report

All tests passed.

` + "```bash" + `
agent-bridge signal QA_PASSED true
` + "```"
}

func (m *MockAgent) managerSignOffResponse() string {
	return `The project looks good. All requirements are met.

` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
` + "```"
}
