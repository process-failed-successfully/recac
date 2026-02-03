package agent

import (
	"context"
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

	// Heuristic: Detect Ticket Generation Request (TPM Agent)
	if strings.Contains(prompt, "Technical Program Manager (TPM)") || strings.Contains(prompt, "generate tickets") {
		return `[
  {
    "id": "PRIMES",
    "title": "Epic: Implement Core Features",
    "description": "Repo: https://github.com/process-failed-successfully/recac-jira-e2e\nImplement the core functionality described in the spec.",
    "type": "Epic",
    "children": [
      {
        "id": "PRIMES-1",
        "title": "Story: Implement Primary Logic",
        "description": "Repo: https://github.com/process-failed-successfully/recac-jira-e2e\nDevelop the main script/application logic.",
        "type": "Story",
        "acceptance_criteria": ["Logic is implemented", "Tests pass"]
      }
    ]
  }
]`, nil
	}

	// 1. Initializer Role
	if strings.Contains(strings.ToLower(prompt), "initializer agent") {
		return `I will initialize the project features.
` + "```bash" + `
echo '{"features": [{"id": "req-1", "description": "impl", "status": "todo"}]}' > feature_list.json
cat feature_list.json | agent-bridge import
` + "```", nil
	}

	// 2. QA Role
	if strings.Contains(prompt, "YOUR ROLE - QA AGENT") {
		return `QA Checks Passed.
` + "```bash" + `
agent-bridge signal QA_PASSED true
` + "```", nil
	}

	// 3. Manager Role
	if strings.Contains(prompt, "YOUR ROLE - PROJECT MANAGER") {
		return `Project Approved.
` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true
` + "```", nil
	}

	// 4. Completion Check (Prevent Loop)
	if strings.Contains(strings.ToLower(prompt), "nothing to commit") {
		return `No changes to commit. Marking features as done to proceed.
` + "```bash" + `
agent-bridge feature list --json | jq -r '.features[].id' | xargs -I {} agent-bridge feature set {} --status done --passes true
agent-bridge signal COMPLETED true
` + "```", nil
	}

	// 5. Default Coding Agent (Smoke Test Logic)
	// If we are in a coding loop (default), generate code and update features.
	// We use a generic approach that works for the smoke test "prime-python" or similar.
	return `I will implement the requested logic and update feature status.
` + "```bash" + `
# Create a dummy implementation file to satisfy requirements
echo "def is_prime(n): return n > 1" > primes.py

# Mark all features as done and passing
agent-bridge feature list --json | jq -r '.features[].id' | xargs -I {} agent-bridge feature set {} --status done --passes true
` + "```", nil
}

// SendStream implements the Agent interface
func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := m.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(resp)
	}
	return resp, err
}

