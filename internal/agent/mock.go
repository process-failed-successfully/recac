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

	// Heuristic 1: TPM (Technical Program Manager)
	if strings.Contains(lowerPrompt, "technical program manager") {
		return `[{"id": "PRIMES", "type": "Task", "title": "Create Prime Number Script", "description": "Calculate primes under 10000", "repo": "https://github.com/process-failed-successfully/recac-jira-e2e"}]`, nil
	}

	// Heuristic 2: Initializer Agent
	if strings.Contains(lowerPrompt, "initializer agent") {
		return "```bash\necho '{\"features\": [{\"id\": \"1\", \"description\": \"Implement prime number calculation\", \"status\": \"todo\"}]}' | agent-bridge import --file -\n```", nil
	}

	// Heuristic 3: QA Agent
	if strings.Contains(lowerPrompt, "qa agent") || strings.Contains(lowerPrompt, "approve or reject") {
		return "QA_PASSED", nil
	}

	// Heuristic 4: Coding Agent (Primes Scenario)
	if strings.Contains(lowerPrompt, "primes.py") || (strings.Contains(lowerPrompt, "prime") && strings.Contains(lowerPrompt, "python")) {
		script := `import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [i for i in range(10000) if is_prime(i)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
`
		// Use heredoc for file creation, then run script, add files, and commit
		// Also mark feature as completed to trigger auto-completion
		cmd := fmt.Sprintf("cat << 'EOF' > primes.py\n%sEOF\n\npython3 primes.py\ngit add -f primes.py primes.json\ngit commit -m \"Add primes script\"\nagent-bridge feature set 1 --status done --passes true", script)
		return fmt.Sprintf("Here is the script.\n```bash\n%s\n```\nTask Completed.", cmd), nil
	}

	// Heuristic 5: Coding Agent (Generic)
	if strings.Contains(lowerPrompt, "coding agent") {
		return "Task Completed", nil
	}

	// Default response
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
