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

	// Check for Initializer Agent
	if strings.Contains(prompt, "## YOUR ROLE - INITIALIZER AGENT") {
		var featuresJSON string

		if strings.Contains(prompt, "[PRIMES]") {
			featuresJSON = `{
  "features": [
    {
      "id": "primes-script",
      "description": "Create primes.py to generate primes [PRIMES]",
      "status": "pending",
      "priority": "MVP",
      "dependencies": { "exclusive_write_paths": ["primes.py"], "read_only_paths": [] }
    }
  ]
}`
		} else {
			featuresJSON = `{
  "features": [
    {
      "id": "project-setup",
      "description": "Initial Project Setup",
      "status": "pending",
      "priority": "MVP",
      "dependencies": { "exclusive_write_paths": ["init.sh"], "read_only_paths": [] }
    }
  ]
}`
		}

		return fmt.Sprintf("```bash\n#!/bin/bash\nset -x\necho \"Mock Initializer: Setting up project...\"\n\n# Create init.sh\ncat << 'EOF' > init.sh\n#!/bin/bash\necho \"Initializing...\"\nEOF\nchmod +x init.sh\n\n# Import Features\ncat << 'EOF' | agent-bridge import\n%s\nEOF\n\n# Git Init\ngit init\ngit config user.email \"agent@recac.io\"\ngit config user.name \"RECAC Agent\"\ngit add .\ngit commit -m \"Initial commit\"\n```", featuresJSON), nil
	}

	// Check for TPM/Ticket generation prompt
	// Note: We check for "Technical Program Manager" which is the role definition.
	// We removed the check for "tickets" because the prompt template might use singular "ticket"
	// or the user might change the phrasing, but the role remains constant.
	if strings.Contains(prompt, "Technical Program Manager") {
		if strings.Contains(prompt, "[PRIMES]") {
			return `[
  {
    "id": "PRIMES",
    "title": "Generate Primes Script [PRIMES]",
    "description": "Create a python script to generate primes. [PRIMES]",
    "type": "Task",
    "status": "TODO"
  }
]`, nil
		}

		// Return a valid JSON list of tickets
		return `[
  {
    "id": "PROJ-1",
    "title": "Initial Project Setup",
    "description": "Set up the initial project structure and dependencies.",
    "type": "Task",
    "status": "TODO"
  }
]`, nil
	}

	// Check for Coding Agent prompt
	if strings.Contains(prompt, "## YOUR ROLE - CODING AGENT") {
		// For the prime-python scenario (detected via [PRIMES] marker)
		if strings.Contains(prompt, "[PRIMES]") {
			script := `#!/bin/bash
set -x

# Configure git
git config user.name "RECAC Agent"
git config user.email "agent@recac.io"

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

primes = [x for x in range(2, 21) if is_prime(x)]
print(f"Primes: {primes}")

with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)
EOF

# Run it
python3 primes.py

# Signal completion
agent-bridge feature set primes-script --status done --passes true
agent-bridge signal COMPLETED true`
			return fmt.Sprintf("\n```bash\n%s\n```\n", script), nil
		}

		// Check for Initial Project Setup (default task)
		if strings.Contains(prompt, "Initial Project Setup") {
			script := `#!/bin/bash
echo "Mock Agent: Completing setup..."
agent-bridge feature set project-setup --status done --passes true
agent-bridge signal COMPLETED true`
			return fmt.Sprintf("\n```bash\n%s\n```\n", script), nil
		}

		// Generic Coding Agent response
		script := `#!/bin/bash
echo "Mock Coding Agent: Working on task..."
ls -la`
		return fmt.Sprintf("\n```bash\n%s\n```\n", script), nil
	}

	// Check for QA Agent
	if strings.Contains(prompt, "## YOUR ROLE - QA AGENT") {
		script := `#!/bin/bash
echo "Mock QA: Verifying..."
agent-bridge signal QA_PASSED true`
		return fmt.Sprintf("\n```bash\n%s\n```\n", script), nil
	}

	// Check for Manager Agent
	if strings.Contains(prompt, "## YOUR ROLE - PROJECT MANAGER") {
		script := `#!/bin/bash
echo "Mock Manager: Approving..."
agent-bridge signal PROJECT_SIGNED_OFF true`
		return fmt.Sprintf("\n```bash\n%s\n```\n", script), nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	// We return a bash script to prevent the "NO-OP LOOP" circuit breaker in tests.
	response := fmt.Sprintf("I will implement the requested features.\n\n```bash\n#!/bin/bash\necho \"%s: Received prompt (%d chars)\"\necho \"Mock Agent executing default action...\"\n```\n\nPrompt preview: %s...",
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
