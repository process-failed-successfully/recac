package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a simple mock agent for testing and mock mode
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

	// Heuristic Logic
	promptLower := strings.ToLower(prompt)

	// 1. Initializer (inject feature list)
	if strings.Contains(prompt, "ROLE - INITIALIZER") {
		return m.handleInitializer(prompt)
	}

	// 2. Project Manager (TPM) - Generate Ticket/Feature List
	if strings.Contains(prompt, "ROLE - PROJECT MANAGER") {
		if strings.Contains(promptLower, "review") || strings.Contains(promptLower, "sign off") {
			return m.handleProjectManagerReview(prompt)
		}
		return m.handleProjectManager(prompt)
	}

	// 3. Coding Agent
	if strings.Contains(prompt, "ROLE - CODING AGENT") {
		return m.handleCodingAgent(prompt)
	}

	// 4. QA Agent
	if strings.Contains(prompt, "ROLE - QA AGENT") || strings.Contains(promptLower, "verify") {
		return m.handleQAAgent(prompt)
	}

	// Completion Signal heuristic (fallback)
	if strings.Contains(prompt, "NONE_ALL_COMPLETE") || strings.Contains(promptLower, "signal completion") {
		return "```bash\nagent-bridge signal COMPLETED true\n```", nil
	}

	// Default fallback
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response.",
		m.responsePrefix, len(prompt))
	return response, nil
}

func (m *MockAgent) handleInitializer(prompt string) (string, error) {
	// Return Bash script to import feature list
	// IDs must match what handleProjectManager generates/expects or what E2E expects
	json := `[{"id":"req-setup-repo","title":"Setup Repository","description":"Initialize git and remote"},{"id":"req-ci-workflow","title":"Setup CI Workflow","description":"Create .github/workflows/ci.yml"},{"id":"req-implement-tests","title":"Implement Tests","description":"Create tests/test_primes.py"},{"id":"req-implement-primes","title":"Implement Primes","description":"Create primes.py"}]`

	script := fmt.Sprintf("```bash\necho '%s' | agent-bridge import\n```", json)
	return script, nil
}

func (m *MockAgent) handleProjectManager(prompt string) (string, error) {
	// Return JSON for CLI/Orchestrator to parse
	// ID:[PRIMES] prefix in title helps identifying the epic context if needed
	json := `[
  {
    "id": "req-setup-repo",
    "title": "ID:[PRIMES] Setup Repository",
    "description": "Initialize git and remote",
    "type": "Task"
  },
  {
    "id": "req-ci-workflow",
    "title": "ID:[PRIMES] Setup CI Workflow",
    "description": "Create .github/workflows/ci.yml",
    "type": "Task"
  },
  {
    "id": "req-implement-tests",
    "title": "ID:[PRIMES] Implement Tests",
    "description": "Create tests/test_primes.py",
    "type": "Task"
  },
  {
    "id": "req-implement-primes",
    "title": "ID:[PRIMES] Implement Primes",
    "description": "Create primes.py",
    "type": "Task"
  }
]`
	return json, nil
}

func (m *MockAgent) handleProjectManagerReview(prompt string) (string, error) {
	return "```bash\nagent-bridge signal PROJECT_SIGNED_OFF true --privileged\n```", nil
}

func (m *MockAgent) handleCodingAgent(prompt string) (string, error) {
	// Check for specific feature IDs
	if strings.Contains(prompt, "req-setup-repo") {
		return "```bash\ngit init\ngit remote add origin https://github.com/example/repo.git || true\ngit fetch || true\ngit checkout -b main || git checkout main || true\n```", nil
	}
	if strings.Contains(prompt, "req-ci-workflow") {
		return `
~~~bash
mkdir -p .github/workflows
cat <<EOF > .github/workflows/ci.yml
name: CI
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Run Tests
        run: python3 -m unittest discover tests
EOF
git add .github/workflows/ci.yml
git commit -m "Add CI workflow" || echo "Nothing to commit"
agent-bridge mark-done req-ci-workflow
~~~
`, nil
	}
	if strings.Contains(prompt, "req-implement-tests") {
		return `
~~~bash
mkdir -p tests
cat <<EOF > tests/test_primes.py
import unittest
from primes import is_prime

class TestPrimes(unittest.TestCase):
    def test_primes(self):
        self.assertTrue(is_prime(2))
        self.assertTrue(is_prime(3))
        self.assertFalse(is_prime(4))
        self.assertTrue(is_prime(5))
EOF
git add tests/test_primes.py
git commit -m "Add tests" || echo "Nothing to commit"
agent-bridge mark-done req-implement-tests
~~~
`, nil
	}
	// Coding Agent heuristic prioritizes checking for specific feature IDs
	// (e.g., req-implement-tests, req-ci-workflow) before checking for req-implement-primes
	// (or generic primes.py strings) to prevent shadowing when prompts mention the implementation file.
	if strings.Contains(prompt, "req-implement-primes") || strings.Contains(prompt, "Feature ID: PRIMES") {
		return `
~~~bash
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
git add primes.py
git commit -m "Add primes implementation" || echo "Nothing to commit"
agent-bridge mark-done req-implement-primes
~~~
`, nil
	}

	// Loop breaker: If "working tree clean" or "nothing to commit" detected, signal QA_PASSED
	if strings.Contains(prompt, "working tree clean") || strings.Contains(prompt, "nothing to commit") {
		return "```bash\nagent-bridge signal QA_PASSED true\n```", nil
	}

	return "I am working on it.", nil
}

func (m *MockAgent) handleQAAgent(prompt string) (string, error) {
	return `
~~~bash
python3 -m unittest discover tests || echo "Tests failed but ignoring for mock"
agent-bridge signal QA_PASSED true
~~~
`, nil
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
