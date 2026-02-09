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

	// Heuristic 1: Jira Ticket Generation (TPM)
	// Trigger: "Technical Program Manager" or "ROLE - PROJECT MANAGER"
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "ROLE - PROJECT MANAGER") {
		// Return valid JSON for ticket generation
		// Format: Array of ticket objects
		// IMPORTANT: Title must contain "ID:[PRIMES]" or similar to match regex extraction if needed
		// For smoke tests, we usually want a simple plan.

		// Check if it's the [PRIMES] scenario
		if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "prime number") {
			return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Create a Python script ` + "`primes.py`" + ` that calculates prime numbers up to 100.",
    "type": "Story",
    "acceptance_criteria": [
      "Script runs without errors",
      "Outputs correct prime numbers"
    ],
    "story_points": 3
  },
  {
    "title": "ID:[PRIMES] Add Unit Tests for Primes",
    "description": "Create ` + "`test_primes.py`" + ` that verifies the generator.",
    "type": "Story",
    "acceptance_criteria": [
      "Tests cover edge cases",
      "All tests pass"
    ],
    "story_points": 2,
    "dependencies": ["ID:[PRIMES] Implement Prime Number Generator"]
  }
]`, nil
		}
	}

	// Heuristic 2: Initializer Role
	// Trigger: "Initialize" or "feature_list.json" import
	// In the smoke test, the first iteration might be initialization if not using pre-imported features.
	// But orchestrator often handles import. The log shows features loaded.
	// If the prompt asks to setup workspace or check files.
	// Often the prompt contains "Feature Tracking" or "feature_list.json".
	// But we need to distinguish from Coding Agent which also sees that.
	// Initializer usually has a specific header if it exists, or it's just the first step.
	// However, in this E2E, the Orchestrator sets up the workspace.
	// If the agent needs to import features, it's done via `agent-bridge import`.
	// For this specific test, we might skip explicit Initializer if not requested.

	// Heuristic 3: Coding Agent (Implementation)
	// Trigger: "YOUR ROLE - CODING AGENT" and "primes.py" or ticket context
	if strings.Contains(prompt, "CODING AGENT") {
		// Task: Implement Prime Number Generator
		if strings.Contains(prompt, "Implement Prime Number Generator") || strings.Contains(prompt, "primes.py") {
			return `I will implement the prime number generator.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [i for i in range(10001) if is_prime(i)]
print(json.dumps({"primes": primes}))
EOF
` + "```" + `

Now I will commit the changes and mark the feature as done.

` + "```bash" + `
git add primes.py
git commit -m "Implement primes.py" || echo "Nothing to commit"
agent-bridge feature set req-primes-py-exists --status done --passes true || true
agent-bridge feature set req-implement-prime-number-script --status done --passes true || true
` + "```", nil
		}

		// Task: Add Unit Tests
		if strings.Contains(prompt, "Add Unit Tests") || strings.Contains(prompt, "test_primes.py") {
			return `I will implement the unit tests.

` + "```bash" + `
cat << 'EOF' > test_primes.py
import unittest
import json
import subprocess

class TestPrimes(unittest.TestCase):
    def test_output_format(self):
        result = subprocess.check_output(['python3', 'primes.py'])
        data = json.loads(result)
        self.assertIn('primes', data)
        self.assertIsInstance(data['primes'], list)
        self.assertIn(2, data['primes'])
        self.assertIn(3, data['primes'])
        self.assertIn(5, data['primes'])

if __name__ == '__main__':
    unittest.main()
EOF
` + "```" + `

Now I will run the tests and commit.

` + "```bash" + `
python3 test_primes.py
git add test_primes.py
git commit -m "Add test_primes.py" || echo "Nothing to commit"
agent-bridge feature set req-implement-tests --status done --passes true || true
agent-bridge qa
` + "```", nil
		}
	}

	// Heuristic 4: QA Agent
	// Trigger: "QA AGENT" or "verify"
	if strings.Contains(prompt, "QA AGENT") {
		return `I will verify the solution.

` + "```bash" + `
python3 test_primes.py
agent-bridge signal QA_PASSED true
` + "```", nil
	}

	// Heuristic 5: Manager Review
	// Trigger: "Project Manager" or "Review" (and not TPM generation)
	if strings.Contains(prompt, "manager_review") || (strings.Contains(prompt, "Project Manager") && !strings.Contains(prompt, "Technical Program Manager") && strings.Contains(prompt, "Review")) {
		return `I have reviewed the work and it looks good.

` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
` + "```", nil
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
