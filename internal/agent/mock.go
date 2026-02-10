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

	upperPrompt := strings.ToUpper(prompt)

	// 1. Initializer Agent
	if strings.Contains(upperPrompt, "INITIALIZER AGENT") || strings.Contains(upperPrompt, "YOU ARE THE INITIALIZER") {
		return `
Here is the initialization plan:

` + "```bash" + `
# Initialize git
git config --global user.email "agent@recac.ai"
git config --global user.name "Recac Agent"
git init
git branch -M main

# Create init.sh
cat << 'EOF' > init.sh
#!/bin/bash
echo "Initializing environment..."
apt-get update && apt-get install -y make python3
EOF
chmod +x init.sh

# Create feature_list.json via agent-bridge import
cat << 'EOF' | agent-bridge import
{
  "project_name": "Mock Project",
  "features": [
    {
      "id": "mock-feature-1",
      "category": "functional",
      "priority": "MVP",
      "description": "A mock feature for testing",
      "status": "pending",
      "steps": ["Step 1"],
      "passes": false,
      "dependencies": {
        "depends_on_ids": [],
        "exclusive_write_paths": [],
        "read_only_paths": []
      }
    }
  ]
}
EOF

# Initial commit
git add .
git commit -m "Initial commit" || echo "Nothing to commit"
# Need to push to ensure E2E tests see the branch
# But we don't know the remote here easily, assuming origin exists or will be added.
# In smoke tests, we might need to push if the test expects remote branch.
# We'll try to push if origin exists.
git push origin main || echo "Push failed (expected if no remote)"
` + "```", nil
	}

	// 2. Technical Program Manager (TPM)
	if strings.Contains(upperPrompt, "TECHNICAL PROGRAM MANAGER (TPM)") {
		return `
` + "```json" + `
[
  {
    "title": "Mock Epic 1",
    "description": "A mock epic for testing",
    "type": "Epic",
    "blocked_by": [],
    "acceptance_criteria": ["Criteria 1"],
    "children": [
      {
        "title": "Mock Story 1",
        "description": "A mock story",
        "type": "Story",
        "blocked_by": [],
        "acceptance_criteria": ["Criteria 1.1"],
        "children": []
      }
    ]
  }
]
` + "```", nil
	}

	// 3. QA Agent
	if strings.Contains(upperPrompt, "QA AGENT") {
		return `
Running QA checks...

` + "```bash" + `
echo "Running tests..."
# Simulate passing tests
agent-bridge signal QA_PASSED true
` + "```", nil
	}

	// 4. Project Manager
	if strings.Contains(upperPrompt, "PROJECT MANAGER") || strings.Contains(upperPrompt, "YOUR ROLE - MANAGER") {
		return `
Reviewing project status... everything looks good.

` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true
agent-bridge signal QA_PASSED true
` + "```", nil
	}

	// 5. Coding Agent
	if strings.Contains(upperPrompt, "CODING AGENT") {
		return `
I will implement the assigned feature.

` + "```bash" + `
# Create a dummy file to simulate work
echo "print('Hello from Mock Agent')" > primes.py

# Run tests (mock)
echo "Tests passed"

# Commit changes
git add .
git commit -m "Implement feature" || echo "Nothing to commit"
git push origin HEAD || echo "Push failed"
` + "```", nil
	}

	// Default fallback response
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
