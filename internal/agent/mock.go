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

	// Handle specific scenarios by inspecting the prompt

	// Scenario: Initializer Agent (Sets up features)
	if strings.Contains(prompt, "## YOUR ROLE - INITIALIZER AGENT") {
		return `
I'll set up the project.

` + "```bash" + `
cat << 'EOF' | agent-bridge import
{
  "project_name": "Primes Project",
  "features": [
    {
      "id": "primes-script",
      "description": "Write a python script that prints primes up to 100.",
      "category": "functional",
      "priority": "MVP",
      "status": "pending",
      "steps": ["Run python script", "Check output"]
    }
  ]
}
EOF

cat << 'EOF' > init.sh
#!/bin/bash
echo "Initializing..."
EOF
chmod +x init.sh
` + "```" + `
`, nil
	}

	// Scenario: QA Agent
	if strings.Contains(prompt, "## YOUR ROLE - QA AGENT") {
		return `
QA Passed.

` + "```bash" + `
agent-bridge signal QA_PASSED true
` + "```" + `
`, nil
	}

	// Scenario: Manager Agent
	if strings.Contains(prompt, "## YOUR ROLE - PROJECT MANAGER") {
		return `
Approved.

` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true
` + "```" + `
`, nil
	}

	// Scenario: Jira Ticket Generation (TPM Agent)
	if strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "Technical Program Manager") {
		return `{
  "epics": [
    {
      "title": "Implement Primes Script",
      "description": "Create a script to print prime numbers.",
      "id": "PRIMES",
      "children": [
        {
          "title": "Write Python Script",
          "description": "Write a python script that prints primes up to 100.",
          "type": "Task",
          "id": "PRIMES-1"
        }
      ]
    }
  ]
}`, nil
	}

	// Scenario: Mock Smoke Test (Hello World)
	if strings.Contains(prompt, "Hello Smoke") {
		return `{
  "epics": [
    {
      "title": "Smoke Test Epic",
      "description": "Epic for smoke testing.",
      "id": "SMOKE",
      "children": [
        {
          "title": "Print Hello Smoke",
          "description": "Write a python script that prints 'Hello Smoke'.",
          "type": "Task",
          "id": "SMOKE-1"
        }
      ]
    }
  ]
}`, nil
	}

	// Scenario: Python Implementation (Coding Agent)
	// Relaxed check for "primes" to catch various prompts
	if strings.Contains(strings.ToLower(prompt), "primes") {
		return `
File: primes.py
` + "```python" + `
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

for i in range(100):
    if is_prime(i):
        print(i)
` + "```" + `

` + "```bash" + `
agent-bridge feature set primes-script --status implemented --passes true
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
