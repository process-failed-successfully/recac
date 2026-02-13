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

	lowerPrompt := strings.ToLower(prompt)

	// 1. Initializer Agent
	// Triggers on "initializer agent"
	if strings.Contains(lowerPrompt, "initializer agent") {
		return "```bash\necho '{\"features\": [{\"id\": \"PRIMES\", \"description\": \"Calculate primes\", \"status\": \"Todo\", \"priority\": \"MVP\", \"passes\": false, \"dependencies\": {\"depends_on_ids\": [], \"exclusive_write_paths\": [], \"read_only_paths\": []}}]}' | agent-bridge import\n```", nil
	}

	// 2. TPM / Ticket Planning
	// Triggers on "Technical Program Manager" or "generate ticket"
	if strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "generate ticket") {
		// Extract repo URL if possible, otherwise use a placeholder
		repoURL := "https://github.com/example/repo"
		if parts := strings.Split(prompt, "Repo: "); len(parts) > 1 {
			// Basic extraction, assumes URL is on the same line or next word
			line := strings.Split(parts[1], "\n")[0]
			repoURL = strings.TrimSpace(line)
		}

		return fmt.Sprintf(`[
  {
    "id": "PRIMES",
    "title": "Implement Prime Number Generator",
    "description": "Create a python script primes.py that prints primes up to 100.",
    "type": "Task",
    "status": "Todo",
    "repo": "%s"
  }
]`, repoURL), nil
	}

	// 3. QA / Manager Review
	// Triggers on "QA Agent" or "Manager Review"
	if strings.Contains(lowerPrompt, "qa agent") || strings.Contains(lowerPrompt, "manager review") {
		return "```bash\nagent-bridge signal QA_PASSED true\n```", nil
	}

	// 4. Coding Agent (Primes Scenario)
	// Triggers on coding keywords. We must ensure we don't trigger this during TPM phase.
	// The TPM prompt usually asks to "generate tickets", whereas the coding prompt asks to "implement".
	// We'll check for "primes.py" or "PRIMES" + absence of "Technical Program Manager" context if possible.
	// Ideally, the system prompt for coding agent identifies it as "You are a software engineer".
	isCoding := strings.Contains(lowerPrompt, "software engineer") || strings.Contains(lowerPrompt, "developer") || !strings.Contains(lowerPrompt, "program manager")

	if (strings.Contains(lowerPrompt, "primes") || strings.Contains(lowerPrompt, "prime number")) && isCoding {
		codeBlock := "```"
		return fmt.Sprintf(`I will implement the prime number generator.

%spython
# primes.py
def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

if __name__ == "__main__":
    for i in range(100):
        if is_prime(i):
            print(i)
%s

%sbash
# Write the file
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

if __name__ == "__main__":
    for i in range(100):
        if is_prime(i):
            print(i)
EOF

# Commit
git add primes.py
git commit -m "feat: implement prime number generator"

# Update feature status
agent-bridge feature set PRIMES --status Done --passes true
%s`, codeBlock, codeBlock, codeBlock, codeBlock), nil
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
