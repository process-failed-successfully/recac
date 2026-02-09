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

	// TPM Heuristic: Jira Ticket Generation
	// Trigger: "Application Specification" (from memory) or context implying ticket creation
	if strings.Contains(prompt, "Application Specification") || strings.Contains(prompt, "TPM") {
		// Return JSON for tickets
		// The smoke test expects valid JSON
		return `[
  {"title": "ID:[PRIMES] Setup Python Project", "description": "Initialize repository structure", "type": "Task", "acceptance_criteria": ["Repo structure created"]},
  {"title": "ID:[PRIMES] Implement Prime Function", "description": "Create primes.py with function", "type": "Task", "acceptance_criteria": ["Implement prime number script", "primes.py exists", "contains exactly 1229 primes"]},
  {"title": "ID:[PRIMES] Implement Tests", "description": "Create test_primes.py", "type": "Task", "acceptance_criteria": ["Tests pass"]},
  {"title": "ID:[PRIMES] CI Workflow", "description": "Create .github/workflows/ci.yml", "type": "Task", "acceptance_criteria": ["Workflow exists"]}
]`, nil
	}

	// Initializer Heuristic
	// Trigger: "INITIALIZER AGENT"
	// Must clone the repo
	if strings.Contains(prompt, "INITIALIZER AGENT") {
		return "```bash\n" + `cat << 'EOF' > feature_list.json
[
  {"id": "req-primes-py-exists", "description": "Implement primes.py"},
  {"id": "req-implement-tests", "description": "Implement tests"},
  {"id": "req-ci-workflow", "description": "CI Workflow"}
]
EOF
agent-bridge import feature_list.json
rm -rf .git .gitignore *
git clone https://github.com/process-failed-successfully/recac-jira-e2e .
` + "\n```", nil
	}

	// Coding Agent Heuristic
	// Trigger: "CODING AGENT"
	if strings.Contains(prompt, "CODING AGENT") {
		// Heuristic to detect primes task
		if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "req-primes-py-exists") {
			return "```bash\n" + `cat << 'EOF' > primes.py
import json

def is_prime(n):
    """Checks if a number is a prime number."""
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

if __name__ == "__main__":
    primes = [x for x in range(10000) if is_prime(x)]
    print(json.dumps({"primes": primes}))
EOF
git add primes.py
git commit -m "feat: add primes.py" || true
agent-bridge feature set req-primes-py-exists completed || echo "Feature req-primes-py-exists not found"
` + "\n```", nil
		}

		if strings.Contains(prompt, "test_primes.py") || strings.Contains(prompt, "req-implement-tests") {
             return "```bash\n" + `cat << 'EOF' > test_primes.py
import unittest
from primes import is_prime

class TestPrimes(unittest.TestCase):
    def test_prime(self):
        self.assertTrue(is_prime(7))
        self.assertFalse(is_prime(4))

if __name__ == '__main__':
    unittest.main()
EOF
git add test_primes.py
git commit -m "test: add test_primes.py" || true
agent-bridge feature set req-implement-tests completed || echo "Feature req-implement-tests not found"
` + "\n```", nil
        }

        if strings.Contains(prompt, "CI Workflow") || strings.Contains(prompt, "req-ci-workflow") {
            return "```bash\n" + `mkdir -p .github/workflows
cat << 'EOF' > .github/workflows/ci.yml
name: CI
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v2
    - name: Set up Python
      uses: actions/setup-python@v2
      with:
        python-version: '3.x'
    - name: Install dependencies
      run: |
        python -m pip install --upgrade pip
    - name: Run tests
      run: python test_primes.py
EOF
git add .github/workflows/ci.yml
git commit -m "ci: add workflow" || true
agent-bridge feature set req-ci-workflow completed || echo "Feature req-ci-workflow not found"
` + "\n```", nil
        }

		// Loop breaker: if nothing specific matched or prompt implies review/cleanup
		// Signal QA Passed if tree is clean (no-op loop breaker)
		if strings.Contains(prompt, "nothing to commit") || strings.Contains(prompt, "working tree clean") {
			return "```bash\n" + `agent-bridge signal QA_PASSED true
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
` + "\n```", nil
		}

		// Fallback for coding agent
		return "```bash\n" + `echo "Coding agent fallback - no specific task detected"
agent-bridge feature set req-ci-workflow completed || true
` + "\n```", nil
	}


	// QA Agent Heuristic
	if strings.Contains(prompt, "QA AGENT") {
		return "```bash\n" + `agent-bridge signal QA_PASSED true` + "\n```", nil
	}

	// Project Manager Heuristic
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return "```bash\n" + `agent-bridge signal PROJECT_SIGNED_OFF true --privileged` + "\n```", nil
	}

	// Default fallback (original behavior)
	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
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
