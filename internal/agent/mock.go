package agent

import (
	"context"
	"fmt"
	"os"
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
// It returns a mock response that acknowledges the prompt or returns structured data based on heuristics
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	lowerPrompt := strings.ToLower(prompt)
	injectedFeatures := strings.ToLower(os.Getenv("RECAC_INJECTED_FEATURES"))

	// Heuristic 0: Planning Phase Detection (must come first)
	isPlanning := strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "tpm")

	// Heuristic 1: Primes Task (Smoke Test)
	// Triggers if prompt or env var mentions "prime" or specific ID
	if strings.Contains(lowerPrompt, "prime") || strings.Contains(lowerPrompt, "primes.json") ||
		strings.Contains(lowerPrompt, "id:[primes]") || strings.Contains(injectedFeatures, "prime") {

		if isPlanning {
			return `[
  {
    "id": "PRIMES-1",
    "title": "Implement Prime Number Generator",
    "description": "Create a Python script that generates prime numbers up to a given limit.",
    "type": "Task",
    "dependencies": []
  },
  {
    "id": "PRIMES-2",
    "title": "Add Unit Tests for Primes",
    "description": "Write unit tests to verify the prime number generator.",
    "type": "Task",
    "dependencies": ["PRIMES-1"]
  }
]`, nil
		} else {
			// Execution Phase: Return Bash script or Completion Message

			// Check for completion first
			// If the prompt contains the success output from the test script, we are done.
			if strings.Contains(lowerPrompt, "ran 2 tests") && strings.Contains(lowerPrompt, "ok") {
				return "Great, the tests passed. Task completed.", nil
			}

			if strings.Contains(lowerPrompt, "test") {
				// Unit Tests
				// IMPORTANT: We include the implementation file creation here as well to ensure
				// the test has its dependencies, as the execution environment might be clean.
				return `Here are the unit tests for the prime number generator.
` + "```bash" + `
cat << 'EOF' > primes.py
def generate_primes(n):
    primes = []
    for num in range(2, n + 1):
        is_prime = True
        for i in range(2, int(num ** 0.5) + 1):
            if num % i == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(num)
    return primes

if __name__ == "__main__":
    print(generate_primes(50))
EOF

cat << 'EOF' > test_primes.py
import unittest
from primes import generate_primes

class TestPrimes(unittest.TestCase):
    def test_primes_up_to_10(self):
        self.assertEqual(generate_primes(10), [2, 3, 5, 7])

    def test_primes_up_to_20(self):
        self.assertEqual(generate_primes(20), [2, 3, 5, 7, 11, 13, 17, 19])

if __name__ == '__main__':
    unittest.main()
EOF
python3 test_primes.py
` + "```", nil
			} else {
				// Implementation
				return `Here is the implementation of the prime number generator.
` + "```bash" + `
cat << 'EOF' > primes.py
def generate_primes(n):
    primes = []
    for num in range(2, n + 1):
        is_prime = True
        for i in range(2, int(num ** 0.5) + 1):
            if num % i == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(num)
    return primes

if __name__ == "__main__":
    print(generate_primes(50))
EOF
` + "```", nil
			}
		}
	}

	// Heuristic 2: Technical Program Manager (TPM) - General
	// Returns a valid JSON list of tickets to satisfy recac CLI
	if isPlanning {
		return `[
  {
    "id": "TASK-1",
    "title": "Initialize Project Structure",
    "description": "Set up the basic project structure and configuration files.",
    "type": "Task",
    "dependencies": []
  },
  {
    "id": "TASK-2",
    "title": "Implement Core Logic",
    "description": "Implement the main functionality of the application.",
    "type": "Task",
    "dependencies": ["TASK-1"]
  }
]`, nil
	}

	// Heuristic 3: QA Agent
	if strings.Contains(lowerPrompt, "qa agent") || strings.Contains(lowerPrompt, "qa_passed") || strings.Contains(lowerPrompt, "you are the qa agent") {
		return "QA_PASSED", nil
	}

	// Heuristic 4: Manager Review
	// Triggers completion of the workflow
	if strings.Contains(lowerPrompt, "manager review") || strings.Contains(lowerPrompt, "project manager") || strings.Contains(lowerPrompt, "qa report") {
		return "agent-bridge signal PROJECT_SIGNED_OFF true --privileged", nil
	}

	// Default: Return a text response
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
