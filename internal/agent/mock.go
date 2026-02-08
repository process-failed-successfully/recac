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

	// Heuristics for Smoke Test Scenarios (MUST be specific to avoid breaking unit tests)

	// 1. TPM Agent (Jira Generation)
	if strings.Contains(prompt, "Technical Program Manager") && strings.Contains(prompt, "[PRIMES]") {
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Implement a Python script ` + "`primes.py`" + ` that generates prime numbers.\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Task",
    "acceptance_criteria": [
      "Script accepts N as argument",
      "Script prints first N primes"
    ],
    "children": []
  }
]`, nil
	}

	// 2. Initializer Agent
	// Matches specific smoke test context (mentioning 'init.sh' and 'Prime' or the specific repo)
	// to avoid intercepting generic initializer tests.
	if strings.Contains(prompt, "YOUR ROLE - INITIALIZER AGENT") && (strings.Contains(prompt, "Prime") || strings.Contains(prompt, "recac-jira-e2e")) {
		return "```bash\n" + `
cat << 'EOF' > feature_list.json
{
  "project_name": "recac-e2e",
  "features": [
    {
      "id": "req-primes-implementation",
      "name": "Implement Prime Number Generator",
      "description": "Implement a Python script primes.py that generates prime numbers.",
      "status": "todo",
      "type": "task",
      "priority": "high",
      "dependencies": []
    }
  ]
}
EOF

# Initialize git if needed
if [ -d ".git" ]; then
    echo "Git already initialized."
else
    if [ -n "$GITHUB_API_KEY" ]; then
        git clone https://x-access-token:$GITHUB_API_KEY@github.com/process-failed-successfully/recac-jira-e2e .
    else
        git init
        git remote add origin https://github.com/process-failed-successfully/recac-jira-e2e
    fi
fi

cat << 'EOF' > init.sh
#!/bin/bash
echo "Initializing environment..."
apt-get update && apt-get install -y python3 make
EOF
chmod +x init.sh
./init.sh

cat << 'EOF' > Makefile
test:
	python3 test_primes.py
EOF

agent-bridge import feature_list.json
` + "\n```", nil
	}

	// 3. Coding Agent
	// STRICT CHECK: Must be the Coding Agent role AND match the Primes scenario.
	if strings.Contains(prompt, "YOUR ROLE - CODING AGENT") && (strings.Contains(prompt, "Prime") || strings.Contains(prompt, "primes.py")) {
		// Extract Feature ID
		re := regexp.MustCompile(`(?m)^\s*-\s*\*\*Feature ID\*\*:\s*([^\s]+)`)
		matches := re.FindStringSubmatch(prompt)
		featureID := "req-primes-implementation" // Default for smoke test
		if len(matches) > 1 {
			extracted := strings.TrimSpace(matches[1])
			// Fix for extracted "Multiple/Not" or empty IDs
			if extracted != "" && !strings.Contains(extracted, "Multiple") {
				featureID = extracted
			}
		}

		return fmt.Sprintf("```bash\n"+`
cat << 'EOF' > primes.py
import sys

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n %% i == 0:
            return False
    return True

def main():
    if len(sys.argv) < 2:
        print("Usage: python3 primes.py <N>")
        sys.exit(1)

    try:
        n = int(sys.argv[1])
    except ValueError:
        print("Error: N must be an integer")
        sys.exit(1)

    count = 0
    num = 2
    primes = []
    while count < n:
        if is_prime(num):\n            primes.append(num)\n            count += 1\n        num += 1\n        \n    print(f"First {n} primes: {primes}")

if __name__ == "__main__":
    main()
EOF

cat << 'EOF' > test_primes.py
import unittest
from primes import is_prime

class TestPrimes(unittest.TestCase):
    def test_is_prime(self):\n        self.assertTrue(is_prime(2))\n        self.assertTrue(is_prime(3))\n        self.assertTrue(is_prime(5))\n        self.assertFalse(is_prime(4))\n        self.assertFalse(is_prime(1))

if __name__ == '__main__':
    unittest.main()
EOF

python3 test_primes.py

agent-bridge feature set %s --status done --passes true
`+"\n```", featureID), nil
	}

	// 4. QA Agent
	if strings.Contains(prompt, "YOUR ROLE - QA AGENT") {
		// Only run real QA commands if it looks like a real project context
		if strings.Contains(prompt, "Makefile") || strings.Contains(prompt, "make test") || strings.Contains(prompt, "recac-e2e") {
			return "```bash\n" + `
make test
agent-bridge signal QA_PASSED true
` + "\n```", nil
		}
	}

	// 5. Manager
	if strings.Contains(prompt, "YOUR ROLE - PROJECT MANAGER") {
		return "```bash\n" + `
agent-bridge signal PROJECT_SIGNED_OFF true
` + "\n```", nil
	}

	// Default Response
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
