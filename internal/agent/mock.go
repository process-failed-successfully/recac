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

	lowerPrompt := strings.ToLower(prompt)

	// Idempotency check: If git commit says "nothing to commit", we are done with this step
	if strings.Contains(lowerPrompt, "nothing to commit") {
		return `
It seems the code is already up to date. I will signal completion.

` + "```bash" + `
agent-bridge signal QA_PASSED true
` + "```" + `
`, nil
	}

	// Heuristic 1: Initializer Agent
	if strings.Contains(lowerPrompt, "initializer agent") {
		return `
I will initialize the project features.

` + "```bash" + `
echo '{"features": [{"id": "feat-1", "description": "Calculate primes", "status": "todo"}]}' | agent-bridge import
` + "```" + `
`, nil
	}

	// Heuristic 2: Technical Program Manager (Planning)
	if strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "generate ticket") {
		// Extract Repo URL if present
		repo := "https://github.com/process-failed-successfully/recac-jira-e2e"
		if strings.Contains(prompt, "Repo: ") {
			parts := strings.Split(prompt, "Repo: ")
			if len(parts) > 1 {
				repo = strings.Split(parts[1], "\n")[0]
				repo = strings.TrimSpace(repo)
			}
		}

		return fmt.Sprintf(`[
  {
    "id": "PRIMES",
    "title": "Implement Prime Number Generator",
    "description": "Repo: %s\n\nCreate a python script primes.py that prints the first 100 prime numbers.",
    "type": "Task",
    "status": "To Do"
  }
]`, repo), nil
	}

	// Heuristic 3a: Manager Review / Project Manager
	// This signals project completion (sign-off)
	if strings.Contains(lowerPrompt, "manager review") || strings.Contains(lowerPrompt, "manager agent") || strings.Contains(lowerPrompt, "project manager") {
		return `
The project meets all requirements and is ready for sign-off.

` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
` + "```" + `
`, nil
	}

	// Heuristic 3b: QA Agent
	// This signals QA completion
	if strings.Contains(lowerPrompt, "qa agent") {
		return `
The code looks good and meets the requirements.

` + "```bash" + `
agent-bridge signal QA_PASSED true
` + "```" + `
`, nil
	}

	// Heuristic 4: Coding Agent (Primes)
	if strings.Contains(lowerPrompt, "primes") || strings.Contains(lowerPrompt, "prime numbers") {
		return `
I will create a python script to generate prime numbers.

` + "```bash" + `
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

count = 0
num = 2
while count < 100:
    if is_prime(num):
        print(num)
        count += 1
    num += 1
EOF

git add primes.py
git commit -m "feat: add prime number generator"
` + "```" + `
`, nil
	}

	// Default fallback
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
