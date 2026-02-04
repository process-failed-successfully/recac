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

	// 1. Check for Initializer (Feature List Import)
	// Must happen before ticket keywords check to avoid false positives
	if strings.Contains(prompt, "feature_list.json") {
		return `
I have analyzed the requirements. Here is the feature list import:

` + "```bash" + `
agent-bridge import <<EOF
{
  "features": [
    {
      "id": "req-primes-py-exists",
      "description": "primes.py exists",
      "status": "pending"
    },
    {
      "id": "req-primes-json-contains-correct-p",
      "description": "primes.json contains correct primes",
      "status": "pending"
    }
  ]
}
EOF
` + "```" + `
`, nil
	}

	// 2. Check for QA Agent
	if strings.Contains(strings.ToUpper(prompt), "ROLE - QA AGENT") {
		return `
I have verified the changes.

` + "```bash" + `
agent-bridge signal QA_PASSED true
` + "```" + `
`, nil
	}

	// 3. Check for Project Manager
	if strings.Contains(strings.ToUpper(prompt), "ROLE - PROJECT MANAGER") || strings.Contains(strings.ToUpper(prompt), "PROJECT MANAGER") {
		return `
Project looks good. Signed off.

` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true
` + "```" + `
`, nil
	}

	// 4. Detect if the prompt expects JSON (simple heuristic for ticket generation)
	// This fixes 'jira generate-from-spec' failing in smoke tests when using mock provider
	// IMPORTANT: This check must happen BEFORE the generic "primes.py" check below,
	// because ticket generation prompts might contain "primes.py" in the requirement description.
	if len(prompt) > 0 && (prompt[0] == '{' || prompt[0] == '[' || containsTicketKeywords(prompt)) {
		return `[
  {
    "title": "Mock Epic",
    "description": "Repo: https://github.com/process-failed-successfully/recac-jira-e2e\nMock description",
    "type": "Epic",
    "children": [
      {
        "title": "Mock Story",
        "description": "Repo: https://github.com/process-failed-successfully/recac-jira-e2e\nMock story description",
        "type": "Story"
      }
    ]
  }
]`, nil
	}

	// 5. Check for Implementation (Primes Scenario)
	// Triggers only if it's NOT a ticket generation request
	if strings.Contains(prompt, "primes.py") {
		return `
I will implement the primes.py script.

` + "```bash" + `
echo "def primes(n):" > primes.py
echo "    pass" >> primes.py
agent-bridge feature set req-primes-py-exists completed
agent-bridge feature set req-primes-json-contains-correct-p completed
` + "```" + `
`, nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func containsTicketKeywords(prompt string) bool {
	// Simple check for keywords likely present in ticket generation prompts
	keywords := []string{"ticket", "jira", "spec", "epic", "story", "Technical Program Manager", "application specification"}
	for _, k := range keywords {
		if strings.Contains(prompt, k) {
			return true
		}
	}
	// Better heuristic: checks if the prompt is asking for a plan
	return len(prompt) > 0 && (strings.Contains(prompt, "generate ticket plan") || strings.Contains(prompt, "app_spec"))
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
