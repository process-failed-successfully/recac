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

	// Heuristic 0: Initializer (Project Setup)
	// Triggers when the system asks to initialize the project
	if strings.Contains(prompt, "## YOUR ROLE - INITIALIZER AGENT") {
		return `
cat << 'EOF' > init.sh
#!/bin/bash
apt-get update
apt-get install -y python3 make
EOF
chmod +x init.sh
./init.sh

cat << 'EOF' > .gitignore
__pycache__/
*.pyc
.DS_Store
EOF

git init
git add .
git commit -m "Initial commit"

cat << 'EOF' | agent-bridge import
{
  "project_name": "Primes",
  "features": [
    {
      "id": "req-primes",
      "category": "functional",
      "priority": "MVP",
      "description": "Implement primes.py",
      "status": "pending",
      "steps": [
        "Run primes.py"
      ],
      "passes": false,
      "dependencies": {
        "depends_on_ids": [],
        "exclusive_write_paths": [],
        "read_only_paths": []
      }
    }
  ]
}
EOF
`, nil
	}

	// Heuristic 1: Initializer (TPM)
	// Triggers when the system asks to break down requirements
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "Break down the requirements") {
		return `[
  {
    "title": "ID:[PRIMES] Prime Number Generator",
    "description": "Implement a prime number generator in Python. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Epic",
    "children": [
      {
        "title": "ID:[req-setup-repo] Initialize Repository",
        "description": "Initialize repository with git. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
        "type": "Story",
        "acceptance_criteria": ["Git initialized"],
        "blocked_by": []
      },
      {
        "title": "ID:[req-implement-primes] Implement Primes",
        "description": "Implement primes.py to calculate prime numbers. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
        "type": "Story",
        "acceptance_criteria": ["primes.py exists", "Calculates primes correctly"],
        "blocked_by": ["ID:[req-setup-repo] Initialize Repository"]
      },
      {
        "title": "ID:[req-implement-tests] Implement Tests",
        "description": "Implement test_primes.py. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
        "type": "Story",
        "acceptance_criteria": ["test_primes.py exists", "Tests pass"],
        "blocked_by": ["ID:[req-implement-primes] Implement Primes"]
      },
      {
        "title": "ID:[req-the-makefile-targets-are-implemented] Create Makefile",
        "description": "Create Makefile with run, test, lint, format targets for primes.py. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
        "type": "Story",
        "acceptance_criteria": ["Makefile exists", "make run works", "make test works"],
        "blocked_by": ["ID:[req-implement-tests] Implement Tests"]
      },
      {
        "title": "ID:[req-ci-workflow] Setup CI",
        "description": "Setup CI workflow. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
        "type": "Story",
        "acceptance_criteria": [".github/workflows/ci.yml exists"],
        "blocked_by": ["ID:[req-the-makefile-targets-are-implemented] Create Makefile"]
      }
    ]
  }
]`, nil
	}

	// Heuristic 2: Project Manager (Directives)
	if strings.Contains(prompt, "## YOUR ROLE - PROJECT MANAGER") {
		// Return completion signal
		return "```bash\nagent-bridge signal COMPLETED true\n```\nAnalysis: All tasks appear to be on track.", nil
	}

	// Heuristic 3: Coding Agent (Implementation)
	if strings.Contains(prompt, "Role: Agent") || strings.Contains(prompt, "## YOUR ROLE - CODING AGENT") {

		// Subtask: Setup Repo
		if strings.Contains(prompt, "req-setup-repo") {
			return `
git init
git add .
git commit -m "Initialize repo"
agent-bridge feature set req-setup-repo --status done --passes true
`, nil
		}

		// Subtask: CI Workflow
		if strings.Contains(prompt, "req-ci-workflow") {
			return `
mkdir -p .github/workflows
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
          if [ -f requirements.txt ]; then pip install -r requirements.txt; fi
      - name: Test
        run: |
          python -m unittest discover
EOF
git add .github
git commit -m "Add CI workflow"
agent-bridge feature set req-ci-workflow --status done --passes true
`, nil
		}

		// Subtask: Implement Tests
		if strings.Contains(prompt, "req-implement-tests") {
			return `
cat << 'EOF' > test_primes.py
import unittest
# Mock import if primes.py doesn't exist yet, but it should
try:
    from primes import is_prime
except ImportError:
    def is_prime(n): return False

class TestPrimes(unittest.TestCase):
    def test_primes(self):
        self.assertTrue(is_prime(2))
        self.assertTrue(is_prime(7))
        self.assertFalse(is_prime(4))
        self.assertFalse(is_prime(1))

if __name__ == '__main__':
    unittest.main()
EOF
git add test_primes.py
git commit -m "Add tests"
agent-bridge feature set req-implement-tests --status done --passes true
`, nil
		}

		// Scenario: Primes (Main Logic or Subtask)
		if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "req-the-makefile-targets-are-implemented") || strings.Contains(prompt, "req-implement-primes") || strings.Contains(prompt, "req-primes") {

			// Determine which ID to signal
			signalID := "req-primes" // Default single task
			if strings.Contains(prompt, "req-the-makefile-targets-are-implemented") {
				signalID = "req-the-makefile-targets-are-implemented"
			} else if strings.Contains(prompt, "req-implement-primes") {
				signalID = "req-implement-primes"
			}

			script := fmt.Sprintf(`
apt-get update
apt-get install -y make

cat << 'EOF' > primes.py
import sys
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n %% i == 0:
            return False
    return True

if __name__ == "__main__":
    primes = []
    for num in range(2, 10000):
        if is_prime(num):
            primes.append(num)

    with open("primes.json", "w") as f:
        json.dump({"primes": primes}, f)
EOF

# Ensure test_primes exists if not created
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

# Commit code
git add primes.py primes.json test_primes.py Makefile
git commit -m "Implement primes and makefile" || echo "Nothing to commit"

agent-bridge feature set %s --status done --passes true
`, signalID)
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
