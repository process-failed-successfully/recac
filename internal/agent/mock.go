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
// It returns a mock response based on simple heuristics
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	lowerPrompt := strings.ToLower(prompt)

	// 1. TPM/Architect Heuristic (JSON)
	// Triggers when asking for JSON and involves TPM/Architect roles
	if strings.Contains(lowerPrompt, "json") && (strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "architect") || strings.Contains(lowerPrompt, "tpm")) {
		// Return a valid JSON structure for ticket generation
		// This schema matches what internal/jira/manager expects
		return `[
			{
				"summary": "Implement Prime Number Generator",
				"description": "Create a Python script that generates prime numbers.",
				"type": "Task",
				"acceptance_criteria": [
					"Script is named primes.py",
					"Prints the first 10 prime numbers"
				],
				"dependencies": [],
				"id": "PRIMES",
				"repo_url": "https://github.com/process-failed-successfully/recac-jira-e2e"
			}
		]`, nil
	}

	// 2. Git Lead Heuristic
	// Triggers when asking for git commands to start work
	if strings.Contains(lowerPrompt, "git lead") {
		return `git checkout -b feature/primes`, nil
	}

	// 3. Coding/Agent Heuristic (primes.py)
	// Triggers for the specific prime number task in smoke tests
	// We want to avoid triggering this for the TPM prompt itself, so we check for exclusion or specific intent
	if (strings.Contains(lowerPrompt, "id:[primes]") || strings.Contains(lowerPrompt, "generate primes") || strings.Contains(lowerPrompt, "primes.json") || (strings.Contains(lowerPrompt, "prime") && strings.Contains(lowerPrompt, "python"))) && !strings.Contains(lowerPrompt, "technical program manager") {
		return `
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = []
num = 2
while len(primes) < 10:
    if is_prime(num):
        primes.append(num)
    num += 1

print(primes)
EOF
`, nil
	}

	// 4. Commit Message Heuristic
	if strings.Contains(lowerPrompt, "commit message") {
		return "feat: Implement primes.py", nil
	}

	// Default: Generic Text Response
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
