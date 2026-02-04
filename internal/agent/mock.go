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

	// Heuristics for Smoke Test Scenarios

	// 1. Ticket Generation (TPM Role)
	// Matches prompt "You are an expert Technical Program Manager..."
	if strings.Contains(prompt, "Technical Program Manager") && strings.Contains(prompt, "tickets") {
		return `[
  {
    "title": "ID:[PRIMES] Create Primes Script",
    "description": "Create a Python script to calculate primes.",
    "type": "Task",
    "status": "To Do",
    "priority": "High"
  }
]`, nil
	}

	// 2. Initializer Agent (Feature List)
	if strings.Contains(prompt, "initializer agent") || strings.Contains(prompt, "feature_list.json") {
		return "```bash\ncat << 'EOF' > feature_list.json\n{\"project_name\":\"test\",\"features\":[{\"id\":\"1\",\"description\":\"test\",\"status\":\"todo\"}]}\nEOF\n```", nil
	}

	// 3. QA Agent
	if strings.Contains(prompt, "QA AGENT") {
		return "```bash\nagent-bridge signal QA_PASSED true\n```", nil
	}

	// 4. Manager Agent
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return "```bash\nagent-bridge signal PROJECT_SIGNED_OFF true\n```", nil
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
