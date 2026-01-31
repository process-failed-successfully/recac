package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a smart mock agent for testing and mock mode.
// It returns structured responses based on prompt triggers to simulate
// the behavior of real agents in E2E scenarios without external API calls.
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
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	lowerPrompt := strings.ToLower(prompt)

	// 1. Ticket Generation (TPM)
	// Trigger: "Type: Task" or explicitly asking for ticket generation
	if strings.Contains(prompt, "Type: Task") || strings.Contains(lowerPrompt, "generate tickets") {
		// Return a valid JSON array of tickets as expected by 'jira generate-from-spec'
		return `[
  {
    "id": "MOCK-1",
    "summary": "Implement Core Feature",
    "description": "Implement the core functionality as requested.",
    "type": "Task",
    "dependencies": []
  },
  {
    "id": "MOCK-2",
    "summary": "Add Tests",
    "description": "Add unit tests for the core feature.",
    "type": "Task",
    "dependencies": ["MOCK-1"]
  }
]`, nil
	}

	// 2. Initialization
	// Trigger: "agent-bridge import" or "Feature List"
	if strings.Contains(lowerPrompt, "agent-bridge import") || strings.Contains(lowerPrompt, "feature list") {
		return "```bash\n# Initialize feature list\necho '{\"features\": [\"mock-feature\"]}' > feature_list.json\nagent-bridge import --file feature_list.json\n```", nil
	}

	// 3. Implementation (Coding)
	// Trigger: "primes.py", "req-primes", "create", "implement"
	// Also check for Mock Story ID to handle specific story implementation
	if strings.Contains(lowerPrompt, "primes.py") || strings.Contains(lowerPrompt, "create") || strings.Contains(prompt, "ID:[MOCK-STORY]") || strings.Contains(prompt, "MOCK-1") {
		return "```bash\n# Create implementation file\ncat <<EOF > primes.json\n{\"primes\": [2, 3, 5, 7, 11]}\nEOF\n\n# Set git config just in case (for CI)\ngit config user.email \"mock@example.com\"\ngit config user.name \"Mock Agent\"\n\n# Commit\ngit add primes.json\ngit commit -m \"Implement primes\"\n```", nil
	}

	// 4. Planning
	if strings.Contains(lowerPrompt, "app_spec.txt") {
		return "I have read the app spec and am ready to proceed.", nil
	}

	// 5. Default / Fallback
	// Return a generic text response that identifies itself as a mock
	// Using a simple string to avoid "invalid character" errors if JSON is NOT expected but leniently parsed
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
