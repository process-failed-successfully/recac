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

	// Heuristics for E2E tests
	if strings.Contains(prompt, "Technical Program Manager") {
		return `[
  {
    "title": "ID:[PRIMES] Prime Number Generator",
    "description": "Implement a Python script to generate prime numbers.",
    "type": "Story",
    "children": []
  }
]`, nil
	}

	if strings.Contains(prompt, "Lead Software Architect") || strings.Contains(prompt, "break down") {
		return `#!/bin/bash
echo '[{"id": "primes", "status": "todo"}]' > feature_list.json
`, nil
	}

	if strings.Contains(prompt, "QA AGENT") {
		return `#!/bin/bash
agent-bridge signal QA_PASSED true || echo "QA signal failed but proceeding"
`, nil
	}

	if strings.Contains(prompt, "Manager Agent") {
		return `#!/bin/bash
agent-bridge signal PROJECT_SIGNED_OFF true || echo "Manager signal failed but proceeding"
`, nil
	}

	// Execution Phase Heuristics
	if strings.Contains(prompt, `"status": "pending"`) || strings.Contains(prompt, `"status":"pending"`) {
		return `#!/bin/bash
# Mock implementation
echo "Running mock implementation for pending task..."
# Update feature list to done
if [ -f feature_list.json ]; then
  sed -i 's/"status": "pending"/"status": "done"/' feature_list.json || sed -i 's/"status":"pending"/"status":"done"/' feature_list.json
fi
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
