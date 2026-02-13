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
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristics for E2E scenarios
	lowerPrompt := strings.ToLower(prompt)

	// 1. Ticket Planning Phase (TPM Role)
	// Trigger on "technical program manager" or "generate ticket"
	if strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "generate ticket") {
		// Extract repo URL from prompt if possible, or use a placeholder
		repoURL := "https://github.com/example/repo"
		if parts := strings.Split(prompt, "Repo: "); len(parts) > 1 {
			repoURL = strings.Split(parts[1], "\n")[0]
		}

		return fmt.Sprintf(`[
  {
    "title": "ID:[PRIMES] Implement Prime Number Python Script",
    "description": "Create a python script named primes.py that calculates primes up to 10000. \n\nRepo: %s\n\nAppSpec:\nruntime: python\n...",
    "type": "Task",
    "acceptance_criteria": [
      "Script calculates primes up to 10000 correctly",
      "Result is printed to stdout"
    ]
  }
]`, repoURL), nil
	}

	// 2. Execution Phase (Coding Agent)
	// Prime Python Scenario - triggers when asked to write code
	// We check for "prime" and "python" BUT NOT "generate ticket" to avoid conflict with planning
	if strings.Contains(lowerPrompt, "prime") && strings.Contains(lowerPrompt, "python") {
		return `I will create a python script to calculate primes.

` + "```bash" + `
cat << 'EOF' > primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

count = 0
for i in range(10000):
    if is_prime(i):
        count += 1
print(f"Found {count} primes")
EOF

git add primes.py
git commit -m "Add primes script"
git push origin HEAD
` + "```", nil
	}

	// Return a mock response that shows the agent received the prompt
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
