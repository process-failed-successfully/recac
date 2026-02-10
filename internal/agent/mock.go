package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a smart mock agent for testing and E2E scenarios.
// It uses heuristics to return appropriate responses (shell commands) based on the prompt role.
type MockAgent struct {
	iterationCount int
}

// NewMockAgent creates a new mock agent
func NewMockAgent() *MockAgent {
	return &MockAgent{}
}

// Send implements the Agent interface
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.iterationCount++
	upperPrompt := strings.ToUpper(prompt)

	// 1. Initializer Agent (Start of project)
	if strings.Contains(upperPrompt, "INITIALIZER AGENT") {
		// Create the initial feature list
		jsonContent := `
{
  "project_name": "mock-project",
  "features": [
    {
      "id": "PRIMES",
      "description": "Implement a function to check if a number is prime.",
      "status": "pending",
      "passes": false
    }
  ]
}
`
		// Escape single quotes for echo
		jsonContent = strings.ReplaceAll(jsonContent, "'", "'\\''")

		cmd := fmt.Sprintf(`echo '%s' > feature_list.json`, jsonContent)
		return m.wrapBash(cmd + "\ngit init && git add . && git commit -m 'Initial commit' || echo 'Init failed'"), nil
	}

	// 2. Coding Agent
	if strings.Contains(upperPrompt, "CODING AGENT") {
		// Simulate doing work and updating the feature status
		// We use a high iteration count check to avoid infinite loops if it gets stuck,
		// but typically it should just work on the first few tries.

		// Create the implementation
		impl := `
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True
`
		cmd := fmt.Sprintf("echo '%s' > primes.py", strings.ReplaceAll(impl, "'", "'\\''"))

		// Mark feature as done
		jsonContent := `
{
  "project_name": "mock-project",
  "features": [
    {
      "id": "PRIMES",
      "description": "Implement a function to check if a number is prime.",
      "status": "done",
      "passes": true
    }
  ]
}
`
		jsonContent = strings.ReplaceAll(jsonContent, "'", "'\\''")
		cmd += fmt.Sprintf("\necho '%s' > feature_list.json", jsonContent)

		// Git operations
		cmd += "\ngit add .\ngit commit -m 'feat: implement primes' || echo 'nothing to commit'\ngit push origin HEAD || echo 'push failed'"

		return m.wrapBash(cmd), nil
	}

	// 3. QA Agent
	if strings.Contains(upperPrompt, "QA AGENT") {
		return m.wrapBash("agent-bridge signal --privileged QA_PASSED true"), nil
	}

	// 4. Manager Agent (Final Review)
	if strings.Contains(upperPrompt, "PROJECT MANAGER") || strings.Contains(upperPrompt, "REVIEW QA REPORT") {
		return m.wrapBash("agent-bridge signal --privileged PROJECT_SIGNED_OFF true"), nil
	}

	// 5. Technical Program Manager (Jira)
	if strings.Contains(upperPrompt, "TECHNICAL PROGRAM MANAGER") {
		// Return a valid JSON structure for tickets if requested
		// The prompt usually asks for a JSON list of tickets.
		return "```json\n[]\n```", nil
	}

	// Default fallback
	return fmt.Sprintf("I received your prompt (%d chars). Mock mode.", len(prompt)), nil
}

// SendStream implements the Agent interface
func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := m.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(resp)
	}
	return resp, err
}

func (m *MockAgent) wrapBash(cmd string) string {
	return fmt.Sprintf("```bash\n%s\n```", cmd)
}
