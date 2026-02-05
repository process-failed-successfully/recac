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

	// Check for Initializer Agent prompt
	// Matches "YOUR ROLE - INITIALIZER AGENT" or just "INITIALIZER AGENT"
	if strings.Contains(prompt, "INITIALIZER AGENT") {
		return `#!/bin/bash
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
`, nil
	}

	// Check for Coding Agent prompt
	if strings.Contains(prompt, "CODING AGENT") {
		// For the prime-python scenario (detected via [PRIMES] marker)
		if strings.Contains(prompt, "[PRIMES]") {
			return `#!/bin/bash
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
agent-bridge feature set req-primes-py-exists --status done --passes true
agent-bridge signal COMPLETED true
`, nil
		}

		// Generic Coding Agent response
		return `#!/bin/bash
echo "Mock Coding Agent: Working on task..."
ls -la
`, nil
	}

	// Fallback Response
	// Sanitize the prompt preview to ensure it doesn't break the markdown block of the response
	// The runner expects a code block to execute commands.

	// Truncate and sanitize
	preview := truncateString(prompt, 100)
	preview = strings.ReplaceAll(preview, "`", "")
	preview = strings.ReplaceAll(preview, "\"", "")
	preview = strings.ReplaceAll(preview, "\n", " ")

	response := fmt.Sprintf("I will implement the requested features.\n\n```bash\n#!/bin/bash\necho \"%s: Received prompt (%d chars)\"\necho \"Prompt preview: %s\"\necho \"Mock Agent executing default action...\"\n```",
		m.responsePrefix, len(prompt), preview)
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
