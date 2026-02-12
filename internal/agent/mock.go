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
	promptLower := strings.ToLower(prompt)

	// 1. TPM Role (Jira Generation)
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") {
		// Return a JSON list of tickets as expected by 'recac jira generate-from-arch'
		// The E2E test often looks for ID:[SCENARIO] tags, so we can check if a scenario is mentioned in the prompt
		// or just return a generic valid structure.
		return `json
[
  {
    "summary": "Implement Core Features",
    "description": "Implement the core functionality described in the spec. ID:[PRIMES]",
    "issuetype": "Story",
    "priority": "High"
  },
  {
    "summary": "Verify Implementation",
    "description": "Verify the implementation matches the spec. ID:[PRIMES]",
    "issuetype": "Task",
    "priority": "Medium"
  }
]
`, nil
	}

	// 2. Initializer Agent
	if strings.Contains(prompt, "INITIALIZER AGENT") || strings.Contains(prompt, "You are an Initializer Agent") {
		return "```bash\n# Mock Initializer\necho 'Initializing project...'\n```", nil
	}

	// 3. Coding Agent
	// Check "prime" as well because the smoke test scenario is "prime-python"
	if strings.Contains(prompt, "Coding Agent") || strings.Contains(promptLower, "coding agent") || strings.Contains(promptLower, "architect agent") || strings.Contains(promptLower, "prime") {
		if strings.Contains(prompt, "Add primes script") || strings.Contains(promptLower, "primes") {
			// Stop looping heuristic
			return "Task Completed", nil
		}
		return "```python\ndef is_prime(n):\n    if n <= 1: return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0:\n            return False\n    return True\n```\nagent-bridge feature set FEATURE-1 --status completed --passes true", nil
	}

	// 4. QA/Manager
	if strings.Contains(prompt, "Approve or Reject") {
		return "QA_PASSED", nil
	}
	if strings.Contains(prompt, "QA Agent") {
		return "LGTM", nil
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
