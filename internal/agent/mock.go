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

	// Heuristics for Infinite Loop Prevention in Tests
	// 1. Initializer: Return a script that writes feature_list.json directly (bypassing agent-bridge)
	if strings.Contains(prompt, "INITIALIZER AGENT") || strings.Contains(prompt, "You are the Initializer") {
		return `
echo "Initializing mock environment..."
mkdir -p .recac/signals
echo '{"project_name":"Mock Project","features":[{"id":"mock-feat","description":"Mock Feature","status":"pending","steps":["Step 1"]}]}' > feature_list.json
echo "Feature list created."
`, nil
	}

	// 2. QA Agent: Signal success directly (bypassing agent-bridge)
	if strings.Contains(prompt, "QA AGENT") || strings.Contains(prompt, "You are the QA Agent") {
		return `
echo "Running QA checks..."
mkdir -p .recac/signals
echo 'true' > .recac/signals/QA_PASSED
echo "QA Passed signal set."
`, nil
	}

	// 3. Manager Agent: Signal sign-off directly
	if strings.Contains(prompt, "PROJECT MANAGER") || strings.Contains(prompt, "You are the Project Manager") {
		return `
echo "Reviewing project..."
mkdir -p .recac/signals
echo 'true' > .recac/signals/PROJECT_SIGNED_OFF
echo "Project Signed Off signal set."
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
