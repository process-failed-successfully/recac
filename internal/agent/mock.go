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

	// Heuristics for E2E scenarios

	// 1. Initializer / Jira Generation (TPM)
	if strings.Contains(prompt, "Technical Program Manager") && (strings.Contains(prompt, "app_spec.txt") || strings.Contains(prompt, "Specification")) {
		return `
[
  {
    "title": "ID:[PRIMES] Implement Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'. The JSON format must have a single key 'primes' containing the list of integers. The script MUST be named 'primes.py'. The output file MUST be named 'primes.json'. Implement prime calculation logic in primes.py, output results to primes.json, validate that the output file contains a 'primes' list, verify that exactly 1229 primes are calculated, and commit primes.json to the repository. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Task",
    "acceptance_criteria": [
      "The script 'primes.py' is implemented and calculates all prime numbers less than 10,000",
      "The output is written to a file named 'primes.json'",
      "The 'primes.json' file contains a single key 'primes' with a list of integers",
      "The list of primes in 'primes.json' contains exactly 1229 primes",
      "The 'primes.json' file is committed to the repository"
    ],
    "children": []
  }
]
`, nil
	}

	// 2. Initializer (Workspace Setup)
	if strings.Contains(prompt, "INITIALIZER") || strings.Contains(prompt, "GET YOUR BEARINGS") {
		return `
cat << 'EOF' > feature_list.json
{
  "project_name": "MFLP-7856",
  "features": [
    {
      "id": "req-the-script-primes-py-is-implem",
      "category": "functional",
      "priority": "critical",
      "description": "The script 'primes.py' is implemented and calculates all prime numbers less than 10,000",
      "status": "pending",
      "passes": false,
      "steps": null,
      "dependencies": {
        "depends_on_ids": null,
        "exclusive_write_paths": null,
        "read_only_paths": null
      }
    },
    {
      "id": "req-the-output-is-written-to-a-fil",
      "category": "functional",\n      "priority": "critical",\n      "description": "The output is written to a file named 'primes.json'",\n      "status": "pending",\n      "passes": false,\n      "steps": null,\n      "dependencies": {\n        "depends_on_ids": null,\n        "exclusive_write_paths": null,\n        "read_only_paths": null\n      }\n    },\n    {\n      "id": "req-the-primes-json-file-contains-",\n      "category": "functional",\n      "priority": "critical",\n      "description": "The 'primes.json' file contains a single key 'primes' with a list of integers",\n      "status": "pending",\n      "passes": false,\n      "steps": null,\n      "dependencies": {\n        "depends_on_ids": null,\n        "exclusive_write_paths": null,\n        "read_only_paths": null\n      }\n    },\n    {\n      "id": "req-the-list-of-primes-in-primes-j",\n      "category": "functional",\n      "priority": "critical",\n      "description": "The list of primes in 'primes.json' contains exactly 1229 primes",\n      "status": "pending",\n      "passes": false,\n      "steps": null,\n      "dependencies": {\n        "depends_on_ids": null,\n        "exclusive_write_paths": null,\n        "read_only_paths": null\n      }\n    }\n  ]\n}\nEOF
agent-bridge import < feature_list.json
`, nil
	}

	// 3. Coding Agent (Primes Implementation)
	if strings.Contains(prompt, "CODING AGENT") && (strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "PRIMES")) {
		// Detect if we've already done the work (prevent loops)
		if strings.Contains(prompt, "nothing to commit") {
			return "agent-bridge signal TRIGGER_QA true --privileged", nil
		}

		return `
cat << 'EOF' > primes.py
def calculate_primes(n):
    primes = []
    for possiblePrime in range(2, n):
        isPrime = True
        for num in range(2, int(possiblePrime ** 0.5) + 1):
            if possiblePrime % num == 0:
                isPrime = False
                break
        if isPrime:
            primes.append(possiblePrime)
    return primes

def main():
    n = 10000
    primes = calculate_primes(n)
    with open('primes.json', 'w') as f:
        import json
        json.dump({"primes": primes}, f)

if __name__ == "__main__":
    main()
EOF

cat << 'EOF' > test_primes.py
import unittest
from primes import calculate_primes

class TestPrimes(unittest.TestCase):
    def test_calculate_primes(self):
        n = 10
        primes = calculate_primes(n)
        self.assertEqual(primes, [2, 3, 5, 7])

if __name__ == "__main__":
    unittest.main()
EOF

python3 primes.py
python3 test_primes.py

agent-bridge feature set req-the-script-primes-py-is-implem --status done --passes true
agent-bridge feature set req-the-output-is-written-to-a-fil --status done --passes true
agent-bridge feature set req-the-primes-json-file-contains- --status done --passes true
agent-bridge feature set req-the-list-of-primes-in-primes-j --status done --passes true

git add .
git commit -m "Implement primes.py - verified end-to-end" || echo "Nothing to commit"
agent-bridge signal TRIGGER_QA true --privileged
`, nil
	}

	// 4. QA Agent
	if strings.Contains(prompt, "QA AGENT") {
		return `
echo "Running QA checks..."
python3 test_primes.py
agent-bridge signal QA_PASSED true --privileged
`, nil
	}

	// 5. Manager Agent
	if strings.Contains(prompt, "PROJECT MANAGER") || strings.Contains(prompt, "Manager") {
		return `
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
agent-bridge signal COMPLETED true --privileged
`, nil
	}

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
