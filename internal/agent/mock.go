package agent

import (
	"context"
	"fmt"
	"regexp"
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

	// TPM / Jira Generation Logic
	if strings.Contains(prompt, "TPM") || strings.Contains(prompt, "Technical Program Manager") {
		// Extract repo URL from prompt
		repoURL := "https://github.com/process-failed-successfully/recac-jira-e2e"
		re := regexp.MustCompile(`Repo: (https?://[^\s]+)`)
		matches := re.FindStringSubmatch(prompt)
		if len(matches) > 1 {
			repoURL = matches[1]
		}

		// Return JSON for tickets
		jsonResp := fmt.Sprintf(`{
  "project_key": "MFLP",
  "project_name": "Mock Flop Project",
  "epics": [
    {
      "title": "Core Features",
      "description": "Essential core features for the application.",
      "stories": [
        {
          "title": "Implement Primes Script ID:[PRIMES-SCRIPT]",
          "description": "Create a Python script named primes.py that calculates prime numbers up to 10,000. The script must be committed to the repository at %s.",
          "priority": "High",
          "story_points": 3
        }
      ]
    }
  ]
}`, repoURL)
		return fmt.Sprintf("Here is the plan:\n```json\n%s\n```", jsonResp), nil
	}

	// Coding Agent Logic (Primes Script)
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "PRIMES") {
		script := `
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [x for x in range(10000) if is_prime(x)]
print(f"Found {len(primes)} prime numbers.")
EOF

git add primes.py
git commit -m "feat: add primes.py"
agent-bridge feature set PRIMES-SCRIPT --status completed --passes true
`
		return fmt.Sprintf("I will create the primes script.\n```bash\n%s\n```", script), nil
	}

	// Initializer Agent Logic
	if strings.Contains(strings.ToLower(prompt), "initializer agent") {
		// Import features using agent-bridge
		features := `{"features": [{"id": "PRIMES-SCRIPT", "title": "Implement Primes Script", "description": "Create primes.py", "status": "todo"}]}`
		script := fmt.Sprintf(`echo '%s' | agent-bridge import --file -`, features)
		return fmt.Sprintf("Initializing features...\n```bash\n%s\n```", script), nil
	}

	// QA Agent Logic
	if strings.Contains(prompt, "Approve or Reject") || strings.Contains(prompt, "QA Agent") {
		script := `agent-bridge signal QA_PASSED true --privileged`
		return fmt.Sprintf("QA passed.\n```bash\n%s\n```", script), nil
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
