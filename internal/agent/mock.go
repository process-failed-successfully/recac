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

	upperPrompt := strings.ToUpper(prompt)

	// Mock Logic for Ticket Generation (TPM Agent)
	// Detects if the prompt is asking to generate tickets from a spec
	if strings.Contains(upperPrompt, "TPM") || (strings.Contains(upperPrompt, "GENERATE") && strings.Contains(upperPrompt, "TICKET")) {
		return `
` + "```json" + `
[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.\n\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Task",
    "acceptance_criteria": [
      "Create primes.py",
      "Output primes.json",
      "Verify 1229 primes"
    ],
    "children": []
  }
]
` + "```", nil
	}

	// Mock Logic for QA Agent (Smoke Tests)
	if strings.Contains(upperPrompt, "QA AGENT") || strings.Contains(upperPrompt, "VERIFY THE PROJECT") {
		return `I have verified the project and it looks good.

` + "```bash" + `
if command -v agent-bridge >/dev/null 2>&1; then
    agent-bridge signal QA_PASSED true
fi
` + "```", nil
	}

	// Mock Logic for Smoke Tests (Task Execution)
	// Only trigger if it's NOT a ticket generation prompt
	if (strings.Contains(upperPrompt, "PRIMES") || strings.Contains(upperPrompt, "PRIME NUMBERS")) && !strings.Contains(upperPrompt, "TPM") {
		return `I will create a python script to calculate prime numbers.

` + "```bash" + `
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

if __name__ == "__main__":
    import sys
    n = int(sys.argv[1]) if len(sys.argv) > 1 else 10
    print([x for x in range(n) if is_prime(x)])
EOF

python3 primes.py 20

git add primes.py
git commit -m "feat: add primes.py" || echo "Nothing to commit"

if command -v agent-bridge >/dev/null 2>&1; then
    agent-bridge signal COMPLETED true
fi
` + "```", nil
	}

	if strings.Contains(upperPrompt, "SPEC") {
		return `I have analyzed the spec.

` + "```bash" + `
if command -v agent-bridge >/dev/null 2>&1; then
    agent-bridge signal COMPLETED true
fi
` + "```", nil
	}

	if strings.Contains(upperPrompt, "NOTHING TO COMMIT") || strings.Contains(upperPrompt, "WORKING TREE CLEAN") {
		return `It seems there is nothing to commit. I will mark the task as completed.

` + "```bash" + `
if command -v agent-bridge >/dev/null 2>&1; then
    agent-bridge signal COMPLETED true
fi
` + "```", nil
	}

	// Default response
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))

	// Ensure valid bash block for tests that expect it
	response += "\n\n```bash\n# No-op command to satisfy runner\necho 'Mock agent received command'\nif command -v agent-bridge >/dev/null 2>&1; then\n    agent-bridge signal COMPLETED true\nfi\n```"

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
