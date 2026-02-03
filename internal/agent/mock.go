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

	// Heuristic 1: Check for QA role
	if strings.Contains(strings.ToUpper(prompt), "QA AGENT") {
		return `Mock QA agent response:

` + "```bash" + `
echo "QA Checks Passed."
agent-bridge signal QA_PASSED true
` + "```" + `
`, nil
	}

	// Heuristic 2: Check for Manager role
	if strings.Contains(strings.ToUpper(prompt), "PROJECT MANAGER") {
		return `Mock Manager response:

` + "```bash" + `
echo "Project Approved."
agent-bridge signal PROJECT_SIGNED_OFF true
` + "```" + `
`, nil
	}

	// Heuristic 3: Check for Implementation request (Prioritize over Initializer)
	// We check for "COMP-1" or "req-feature-works" or "Calculate primes" (common in e2e)
	if strings.Contains(prompt, "COMP-1") || strings.Contains(prompt, "req-feature-works") || strings.Contains(prompt, "Implement Core Feature") || strings.Contains(prompt, "Calculate primes") || strings.Contains(prompt, "primes.py") {
		return `Mock agent implementation:

` + "```bash" + `
echo "Implementing Feature..."
# Simulate work
echo "Work done."
# Mark feature as complete (using 'update' alias for compatibility)
agent-bridge feature set req-feature-works --status done --passes true || agent-bridge feature set req-primes-json-contains-correct-p --status done --passes true
` + "```" + `
`, nil
	}

	// Heuristic 4: Check for Initializer Logic (Ticket Generation)
	// Must contain "feature_list.json" AND ("INITIALIZER" or "Initialize") to be specific.
	if strings.Contains(prompt, "feature_list.json") && (strings.Contains(strings.ToUpper(prompt), "INITIALIZER") || strings.Contains(prompt, "Initialize")) {
		return `Mock Initializer Response:

` + "```bash" + `
cat << 'EOF' | agent-bridge import
{
  "project_name": "Mock System",
  "features": [
    {
      "id": "req-feature-works",
      "category": "functional",
      "priority": "MVP",
      "description": "Feature works",
      "status": "pending",
      "steps": ["Verify feature"],
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
echo "Initialized features."
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
