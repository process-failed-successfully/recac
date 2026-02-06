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

	// Heuristic 1: Initializer (TPM)
	// Triggers when the system asks to break down requirements
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "Break down the requirements") {
		return `{
  "project_name": "prime-python",
  "features": [
    {
      "id": "req-setup-repo",
      "category": "core",
      "priority": "MVP",
      "description": "Initialize repository with git",
      "status": "pending",
      "passes": false,
      "steps": ["git init"],
      "dependencies": {}
    },
    {
      "id": "req-implement-primes",
      "category": "core",
      "priority": "MVP",
      "description": "Implement primes.py to calculate prime numbers",
      "status": "pending",
      "passes": false,
      "steps": ["Create primes.py"],
      "dependencies": {"depends_on_ids": ["req-setup-repo"]}
    },
    {
      "id": "req-implement-tests",
      "category": "core",
      "priority": "MVP",
      "description": "Implement test_primes.py",
      "status": "pending",
      "passes": false,
      "steps": ["Create test_primes.py"],
      "dependencies": {"depends_on_ids": ["req-implement-primes"]}
    },
    {
      "id": "req-the-makefile-targets-are-implemented",
      "category": "core",
      "priority": "MVP",
      "description": "Create Makefile with run, test, lint, format targets",
      "status": "pending",
      "passes": false,
      "steps": ["Create Makefile"],
      "dependencies": {"depends_on_ids": ["req-implement-tests"]}
    },
    {
      "id": "req-ci-workflow",
      "category": "core",
      "priority": "MVP",
      "description": "Setup CI workflow",
      "status": "pending",
      "passes": false,
      "steps": ["Create .github/workflows/ci.yml"],
      "dependencies": {"depends_on_ids": ["req-the-makefile-targets-are-implemented"]}
    }
  ]
}`, nil
	}

	// Heuristic 2: Project Manager (Directives)
	if strings.Contains(prompt, "## YOUR ROLE - PROJECT MANAGER") {
		// Return completion signal
		return "```bash\nagent-bridge signal COMPLETED true\n```\nAnalysis: All tasks appear to be on track.", nil
	}

	// Heuristic 3: Coding Agent (Implementation)
	if strings.Contains(prompt, "Role: Agent") || strings.Contains(prompt, "## YOUR ROLE - CODING AGENT") {
		// Scenario: Primes
		if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "req-the-makefile-targets-are-implemented") {
			script := `
apt-get update
apt-get install -y make

cat << 'EOF' > primes.py
import sys

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

if __name__ == "__main__":
    # Print first 10 primes
    count = 0
    num = 2
    while count < 10:
        if is_prime(num):
            print(num)
            count += 1
        num += 1
EOF

cat << 'EOF' > test_primes.py
import unittest
from primes import is_prime

class TestPrimes(unittest.TestCase):
    def test_primes(self):
        self.assertTrue(is_prime(2))
        self.assertTrue(is_prime(7))
        self.assertFalse(is_prime(4))
        self.assertFalse(is_prime(1))

if __name__ == '__main__':
    unittest.main()
EOF

cat << 'EOF' > Makefile
run:
	@echo "Running primes.py"
	python3 primes.py

test:
	@echo "Running test_primes.py"
	python3 test_primes.py

lint:
	@echo "Running pylint"
	# Mock pylint
	echo "pylint passed"

format:
	@echo "Running autopep8"
	echo "autopep8 passed"

init:
	@echo "Initializing environment"
	apt-get update
	apt-get install -y python3 make
EOF

make run
make test

agent-bridge feature set req-the-makefile-targets-are-implemented --status done --passes true
`
			return fmt.Sprintf("I will implement the requested changes.\n\n```bash\n%s\n```", script), nil
		}

		// Fallback command to avoid NO-OP loop
		return "```bash\necho 'Mock Agent is thinking...'\n```", nil
	}

	// Default fallback
	return fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100)), nil
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
