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

	// Heuristic for Ticket Generation (TPM Agent)
	// If the prompt asks for a ticket plan (TPM role), return a valid JSON structure.
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "ticket plan") {
		return `[
  {
    "title": "ID:[SETUP] Initial Project Setup",
    "description": "Set up the repository and development environment.",
    "type": "Epic",
    "acceptance_criteria": [
      "Repository created",
      "Dependencies installed"
    ],
    "children": [
      {
        "title": "Initialize Repository",
        "description": "Run git init and create initial commit.",
        "type": "Story"
      }
    ]
  }
]`, nil
	}

	// Heuristic for Initializer Agent
	// If the prompt is for the Initializer (asks for feature_list.json), return a bash block to create it.
	if strings.Contains(prompt, "INITIALIZER AGENT") || strings.Contains(prompt, "Create feature_list.json") {
		return `I will set up the project foundation.

` + "```bash" + `
cat << 'EOF' | agent-bridge import
{
  "project_name": "Mock Project",
  "features": [
    {
      "id": "init-setup",
      "category": "core",
      "priority": "MVP",
      "description": "Initial Setup",
      "status": "pending",
      "steps": ["Step 1: Check init"],
      "passes": false
    }
  ]
}
EOF

cat > init.sh << 'EOF'
#!/bin/bash
echo "Project Initialized"
EOF
chmod +x init.sh
` + "```" + `

And the initial commit:

` + "```bash" + `
git init
git config user.email "agent@recac.ai"
git config user.name "Recac Agent"
git add .
git commit -m "Initial commit"
` + "```", nil
	}

	// Heuristic for Coding Agent / Generic
	// If it's a generic prompt asking for code or tasks, return a simple bash command to prevent No-Op loop.
	// We check for typical prompt markers.
	if strings.Contains(prompt, "Coding Agent") || strings.Contains(prompt, "## YOUR ROLE") {
		return `I am working on the task.

` + "```bash" + `
echo "Executing mock agent task..."
ls -la
` + "```", nil
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
