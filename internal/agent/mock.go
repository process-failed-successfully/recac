package agent

import (
	"context"
	"encoding/json"
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

	// Normalize prompt for case-insensitive matching
	upperPrompt := strings.ToUpper(prompt)

	// TPM (Technical Program Manager) - Ticket Generation
	if strings.Contains(upperPrompt, "TECHNICAL PROGRAM MANAGER") {
		tickets := []map[string]interface{}{
			{
				"id":          "PRIMES",
				"title":       "Implement Primes",
				"description": "Create a python script that prints prime numbers up to 100",
				"type":        "Task",
			},
		}
		data, _ := json.Marshal(tickets)
		return string(data), nil
	}

	// Initializer Agent - Feature List
	if strings.Contains(upperPrompt, "INITIALIZER AGENT") {
		// Return valid bash block calling agent-bridge import
		// We use a bash block because Initializer is instructed to use agent-bridge import
		return `I will initialize the feature list.

` + "```bash" + `
cat <<EOF | agent-bridge import
{
  "project_name": "MFLP-9907",
  "features": [
    {
      "id": "prime-numbers",
      "description": "Print primes to 100",
      "status": "todo",
      "passes": false,
      "priority": "MVP",
      "category": "functional",
      "steps": ["Run python script", "Check output"],
      "dependencies": {
          "exclusive_write_paths": ["primes.py"],
          "read_only_paths": []
      }
    }
  ]
}
EOF
` + "```" + `
`, nil
	}

	// Loop Breaker / QA Check
	// If the system says "nothing to commit", it means the previous step (implementation) is done and committed.
	// We should signal success to move forward.
	if strings.Contains(upperPrompt, "NOTHING TO COMMIT") || strings.Contains(upperPrompt, "WORKING TREE CLEAN") {
		// We MUST also mark the feature as done, otherwise premature sign-off guardrail will reject it.
		return `It looks like the work is done and clean. I will mark the feature as done and signal completion.

` + "```bash" + `
agent-bridge feature set prime-numbers --status done --passes true
agent-bridge signal --privileged QA_PASSED true
agent-bridge signal --privileged PROJECT_SIGNED_OFF true
` + "```" + `
`, nil
	}

	// Coding Agent - Implementation
	if strings.Contains(upperPrompt, "PRIME NUMBERS") || strings.Contains(upperPrompt, "IMPLEMENT PRIMES") || strings.Contains(upperPrompt, "PRIME-NUMBERS") || strings.Contains(upperPrompt, "PRINT PRIMES") {
		// Return a bash command to create the file, so the agent actually performs an action
		// and avoids tripping the NO-OP loop circuit breaker.
		return `I will create the python script.

` + "```bash" + `
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

for i in range(1, 101):
    if is_prime(i):
        print(i)
EOF
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
