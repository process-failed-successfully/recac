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

	// 1. Initializer Agent
	if strings.Contains(prompt, "ROLE - INITIALIZER") {
		return m.handleInitializer(prompt), nil
	}

	// 2. Coding Agent
	if strings.Contains(prompt, "ROLE - CODING AGENT") {
		return m.handleCodingAgent(prompt), nil
	}

	// 3. Project Manager / Manager Review
	if strings.Contains(prompt, "ROLE - PROJECT MANAGER") {
		return m.handleProjectManager(prompt), nil
	}

	// Default / Fallback
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))

	// Add a harmless bash command to prevent NO-OP detection
	response += "\n\n```bash\necho \"Mock Agent Fallback\"\n```"

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

// --- Specific Handlers ---

func (m *MockAgent) handleInitializer(prompt string) string {
	// Generate a script to initialize the repo and feature list
	// We use `git commit --allow-empty` to ensure HEAD exists.
	// We use `cat << 'EOF' | agent-bridge import` to create features.

	script := `
git config --global user.email "agent@recac.io"
git config --global user.name "RECAC Agent"
git init
git commit --allow-empty -m "Initial commit"

cat << 'EOF' > init.sh
#!/bin/bash
echo "Initializing environment..."
EOF
chmod +x init.sh

cat << 'EOF' | agent-bridge import
{
  "project_name": "Mock Project",
  "features": [
    {
      "id": "mock-feature-1",
      "category": "functional",
      "priority": "MVP",
      "description": "Implement a mock service",
      "status": "pending",
      "steps": ["Run service"],
      "passes": false,
      "dependencies": {
        "depends_on_ids": [],
        "exclusive_write_paths": ["."],
        "read_only_paths": []
      }
    }
  ]
}
EOF
echo "Feature list initialized"
`
	return fmt.Sprintf("Here is the initialization script:\n\n```bash%s\n```", script)
}

func (m *MockAgent) handleCodingAgent(prompt string) string {
	// Detect if we are assigned a task
	// The prompt usually contains "Your assigned task is {task_id}"

	// If the prompt asks to implement mock-feature-1 (or just generally), do it.

	script := `
echo "Implementing mock feature..."
echo "package main; func main() { println(\"Hello\") }" > main.go
git add main.go
git commit -m "Implement mock feature"
agent-bridge feature set mock-feature-1 --status done --passes true
`
	return fmt.Sprintf("I will implement the feature.\n\n```bash%s\n```", script)
}

func (m *MockAgent) handleProjectManager(prompt string) string {
	// Check if features are done.
	// The prompt includes the QA report or feature status.
	// If the prompt indicates success/done, sign off.

	// Simple heuristic: If prompt contains "passes: true" or "QA_PASSED=true" or we assume success in mock sequence.
	// However, the prompt history might just be text.
	// Let's assume if we are called, we check if we should approve.
	// In the smoke test, we want to succeed eventually.
	// If the prompt shows "pending" features, we should wait.

	if strings.Contains(prompt, "pending") {
		// Reject / Wait
		return "Features are still pending.\n\n```bash\nagent-bridge signal COMPLETED false\n```"
	}

	// Approve
	return "Excellent work.\n\n```bash\nagent-bridge signal PROJECT_SIGNED_OFF true\n```"
}
