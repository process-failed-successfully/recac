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

	// Heuristics for Smoke Test (prime-python scenario)

	// 1. TPM Agent: Generates the plan
	// The prompt starts with "You are an expert Technical Program Manager (TPM)..."
	// We relax the check to just the role title to be robust.
	if strings.Contains(prompt, "Technical Program Manager (TPM)") || strings.Contains(prompt, "ROLE - TECHNICAL PROGRAM MANAGER") {
		// TPM expects JSON. The CLI handles markdown wrapping, so we provide it for consistency.
		return "```json\n[\n  {\n    \"id\": \"req-primes\",\n    \"title\": \"Implement Prime Number Function\",\n    \"description\": \"Create a Python file primes.py that checks if a number is prime. Repo: https://github.com/process-failed-successfully/recac-jira-e2e\",\n    \"status\": \"todo\",\n    \"type\": \"task\"\n  }\n]\n```", nil
	}

	// 2. Initializer: Imports the features
	if strings.Contains(prompt, "ROLE - INITIALIZER") {
		return "```bash\ncat <<EOF > feature_list.json\n{\n  \"project_name\": \"primes\",\n  \"features\": [\n    {\n      \"id\": \"req-primes\",\n      \"description\": \"Create a Python file primes.py that checks if a number is prime.\",\n      \"status\": \"pending\",\n      \"type\": \"feature\"\n    }\n  ]\n}\nEOF\nagent-bridge import < feature_list.json\n```", nil
	}

	// 3. Coding Agent: Implements the code
	if strings.Contains(prompt, "ROLE - CODING AGENT") {
		// Detect completion (loop breaker)
		if strings.Contains(prompt, "nothing to commit") || strings.Contains(prompt, "working tree clean") {
			return "```bash\nagent-bridge feature set req-primes --status done\n```", nil
		}

		// Detect specific task
		if strings.Contains(prompt, "req-primes") || strings.Contains(prompt, "primes.py") {
			return "```bash\ncat <<EOF > primes.py\ndef is_prime(n):\n    if n <= 1:\n        return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0:\n            return False\n    return True\nEOF\ngit add primes.py\ngit commit -m \"Add primes.py\" || true\ngit push origin HEAD:refs/heads/feature/req-primes --force\nagent-bridge feature set req-primes --status done\n```", nil
		}
	}

	// 4. Project Manager: Signs off
	if strings.Contains(prompt, "ROLE - PROJECT MANAGER") {
		return "```bash\nagent-bridge signal --privileged PROJECT_SIGNED_OFF true\n```", nil
	}

	// 5. QA: Passes
	if strings.Contains(prompt, "ROLE - QA") {
		return "```bash\nagent-bridge signal create QA_PASSED true\n```", nil
	}

	// Default Mock Response
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
