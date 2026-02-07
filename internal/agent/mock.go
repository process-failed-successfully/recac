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

	// Check for TPM/Ticket generation prompt
	if strings.Contains(prompt, "Technical Program Manager") {
		// Handle Primes scenario for TPM
		if strings.Contains(prompt, "[PRIMES]") {
			return `[
  {
    "id": "PRIMES",
    "title": "[PRIMES] Create Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.",
    "type": "Task",
    "status": "TODO"
  }
]`, nil
		}

		// Return a valid JSON list of tickets (Default)
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

	// Check for Initializer Agent prompt
	if strings.Contains(prompt, "YOUR ROLE - INITIALIZER AGENT") {
		// Handle Primes scenario in Initializer
		if strings.Contains(prompt, "[PRIMES]") {
			return "```bash\n" + `#!/bin/bash
set -x
git init
git config user.name "RECAC Agent"
git config user.email "agent@recac.io"

# Import feature list for Primes scenario
cat << 'EOF' | agent-bridge import
{
  "project_name": "Prime Python Project",
  "features": [
    {
      "id": "req-primes-py-exists",
      "category": "functional",
      "priority": "MVP",
      "description": "[PRIMES] Create Prime Number Script",
      "status": "pending",
      "steps": ["Create primes.py", "Run script", "Verify primes.json"],
      "passes": false,
      "dependencies": {}
    }
  ]
}
EOF
` + "\n```", nil
		}

		return "```bash\n" + `#!/bin/bash
set -x
git init
git config user.name "RECAC Agent"
git config user.email "agent@recac.io"

# Import a dummy feature list to satisfy the runner
cat << 'EOF' | agent-bridge import
{
  "project_name": "Mock Project",
  "features": [
    {
      "id": "init-task",
      "category": "functional",
      "priority": "MVP",
      "description": "Initial Setup",
      "status": "pending",
      "steps": ["Step 1"],
      "passes": false,
      "dependencies": {}
    }
  ]
}
EOF
` + "\n```", nil
	}

	// Check for Coding Agent prompt
	// Note: We MUST exclude QA agent prompts that might contain "Coding Agent" references in their context/history
	// but the header check "## YOUR ROLE - CODING AGENT" usually handles this.
	// However, to be safe, we check for QA Role first or rely on the strict header.
	if strings.Contains(prompt, "## YOUR ROLE - CODING AGENT") {
		// For the prime-python scenario
		// Detect via [PRIMES] marker OR the specific feature ID that the Initializer creates
		if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "req-primes-py-exists") {
			return "```bash\n" + `#!/bin/bash
set -x

# Configure git
git config user.name "RECAC Agent"
git config user.email "agent@recac.io"

# Create primes.py
# Note: E2E test requires primes < 10000.
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

# Calculate primes < 10000 to match E2E verification
primes = [x for x in range(2, 10000) if is_prime(x)]
print(f"Calculated {len(primes)} primes")

with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)
EOF

# Run it
python3 primes.py

# Signal completion
agent-bridge feature set req-primes-py-exists --status done --passes true
agent-bridge signal COMPLETED true
` + "\n```", nil
		}

		// Generic Coding Agent response
		return "```bash\n" + `#!/bin/bash
echo "Mock Coding Agent: Working on task..."
ls -la
` + "\n```", nil
	}

	// Check for QA Agent prompt
	if strings.Contains(prompt, "## YOUR ROLE - QA AGENT") {
		return "```bash\n" + `#!/bin/bash
set -x
echo "Mock QA Agent: Verifying..."
# Signal QA success
agent-bridge signal QA_PASSED true
` + "\n```", nil
	}

	// Check for Project Manager prompt (Sign-off)
	if strings.Contains(prompt, "## YOUR ROLE - PROJECT MANAGER") {
		return "```bash\n" + `#!/bin/bash
set -x
echo "Mock Manager: Signing off..."
agent-bridge signal PROJECT_SIGNED_OFF true
` + "\n```", nil
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
