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

	// Heuristic 1: Detect TPM Ticket Generation
	if strings.Contains(prompt, "Technical Program Manager") {
		return `[
  {
    "title": "Setup repository structure",
    "description": "Initialize the project repository with standard layout (cmd, internal, pkg).",
    "type": "Task",
    "acceptance_criteria": [
      "Repository created",
      "Standard folders exist"
    ]
  },
  {
    "title": "Implement core logic",
    "description": "Develop the main business logic for the application.",
    "type": "Story",
    "acceptance_criteria": [
      "Core functions implemented",
      "Unit tests passing"
    ]
  }
]`, nil
	}

	// Heuristic 2: Detect Coding Agent
	if strings.Contains(prompt, "CODING AGENT") {
		return "Here is the implementation:\n```bash\n" + `
echo "Mock Coding Agent executing..."
# Simulate work
echo "package main" > main.go
echo "func main() {}" >> main.go
# Commit changes
git add .
git commit -m "feat: implement core logic" || echo "Nothing to commit"
` + "\n```", nil
	}

	// Heuristic 3: Detect QA Agent
	if strings.Contains(prompt, "QA AGENT") {
		return "Running QA:\n```bash\n" + `
echo "Running QA checks..."
# Simulate successful QA
echo "QA Passed"
agent-bridge signal QA_PASSED true
` + "\n```", nil
	}

	// Heuristic 4: Detect Project Manager (Sign-off)
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return "Signing off:\n```bash\n" + `
echo "Reviewing project..."
# Simulate sign-off
agent-bridge signal PROJECT_SIGNED_OFF true
` + "\n```", nil
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
