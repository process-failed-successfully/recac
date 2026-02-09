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

	// 1. Initializer Heuristic
	if strings.Contains(prompt, "CREATE FEATURE_LIST.JSON") {
		return `
cat <<EOF > feature_list.json
[
  {
    "id": "req-git-initialized",
    "description": "Initialize git repository",
    "type": "Task",
    "status": "TODO"
  },
  {
    "id": "req-primes-py-exists",
    "description": "Create primes.py",
    "type": "Task",
    "status": "TODO"
  },
  {
    "id": "req-script-prints-primes",
    "description": "Script prints primes",
    "type": "Task",
    "status": "TODO"
  }
]
EOF
agent-bridge import < feature_list.json
`, nil
	}

	// 2. Loop Breaker / Project Manager Completion
	upperPrompt := strings.ToUpper(prompt)
	if strings.Contains(upperPrompt, "NOTHING TO COMMIT") ||
	   strings.Contains(upperPrompt, "WORKING TREE CLEAN") ||
	   strings.Contains(upperPrompt, "EVERYTHING UP-TO-DATE") {
		return "agent-bridge signal --privileged QA_PASSED true && agent-bridge signal --privileged PROJECT_SIGNED_OFF true", nil
	}

	// 3. Technical Program Manager (TPM)
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") || strings.Contains(upperPrompt, "PROJECT MANAGER") {
		// Extract repo URL for description if present
		repoURL := "https://example.com/repo"
		re := regexp.MustCompile(`Repo: (https?://\S+)`)
		match := re.FindStringSubmatch(prompt)
		if len(match) > 1 {
			repoURL = match[1]
		}

		// Return JSON for Jira ticket generation
		return fmt.Sprintf(`
[
  {
    "summary": "Implement Prime Number Script",
    "description": "Create a python script to print prime numbers. Repo: %s",
    "issuetype": "Task",
    "priority": "High",
    "acceptance_criteria": [
      "req-primes-py-exists",
      "req-script-prints-primes"
    ]
  }
]
`, repoURL), nil
	}

	// 4. Coding Agent
	if strings.Contains(upperPrompt, "CODING AGENT") || strings.Contains(prompt, "primes.py") {
		return `
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

for i in range(1, 101):
    if is_prime(i):
        print(i)
EOF

cat <<EOF > test_primes.py
import unittest
from primes import is_prime

class TestPrimes(unittest.TestCase):
    def test_is_prime(self):
        self.assertTrue(is_prime(7))
        self.assertFalse(is_prime(4))

if __name__ == '__main__':
    unittest.main()
EOF
`, nil
	}

	// 5. QA Agent
	if strings.Contains(upperPrompt, "QA AGENT") {
		return `
python3 test_primes.py
agent-bridge signal --privileged QA_PASSED true
`, nil
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
