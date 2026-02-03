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

	// Role detection (Case insensitive)
	isQA := containsIgnoreCase(prompt, "QA AGENT") || containsIgnoreCase(prompt, "QA_AGENT")
	isManager := containsIgnoreCase(prompt, "Manager Agent") || containsIgnoreCase(prompt, "Manager")
	// "Role: Manager" or similar contexts
	isManager = isManager || (containsIgnoreCase(prompt, "Role") && containsIgnoreCase(prompt, "Manager"))

	// Implementation detection
	isImpl := containsIgnoreCase(prompt, "PRIMES") || containsIgnoreCase(prompt, "Prime Number Generator")

	// Priority: Role > Impl (to avoid confusion if prompt contains both context)

	// QA Logic
	if isQA {
		return "QA Passed.\n\n```bash\nagent-bridge signal set QA_PASSED\n```", nil
	}

	// Manager Logic
	if isManager {
		return "Project Signed Off.\n\n```bash\nagent-bridge signal set PROJECT_SIGNED_OFF\n```", nil
	}

	// Heuristic for Jira Ticket Generation (TPM Agent)
	if isTPMPrompt(prompt) {
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Implement a Python script that generates prime numbers.\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Story",
    "acceptance_criteria": [
      "Create primes.py",
      "Implement verify_prime function",
      "Add unit tests"
    ],
    "children": []
  }
]`, nil
	}

	// Implementation Logic (Primes)
	// We check this last among roles, but before generic response.
	// But ensure we don't trigger this for Manager/QA if they mention "PRIMES" in context.
	// (The Role checks above should handle that, as they return early).
	if isImpl {
		return "I will implement the prime number generator.\n\n```bash\n" +
			"echo 'Implementing primes.py'\n" +
			"cat <<EOF > primes.py\n" +
			"def is_prime(n):\n" +
			"    if n <= 1: return False\n" +
			"    for i in range(2, int(n**0.5) + 1):\n" +
			"        if n % i == 0: return False\n" +
			"    return True\n" +
			"EOF\n\n" +
			"# Mark features as done\n" +
			"agent-bridge feature list --json | jq -r '.features[].id' | xargs -I {} agent-bridge feature set {} --status done --passes true\n" +
			"```", nil
	}

	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func isTPMPrompt(prompt string) bool {
	p := truncateString(prompt, 200) // Optimization
	return (containsIgnoreCase(p, "Technical Program Manager") || containsIgnoreCase(p, "TPM"))
}

func containsIgnoreCase(s, substr string) bool {
	s, substr = strings.ToLower(s), strings.ToLower(substr)
	return strings.Contains(s, substr)
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
