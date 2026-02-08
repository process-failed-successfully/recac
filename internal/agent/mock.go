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

	// Heuristics for Smoke Test Scenarios

	// 1. Initializer Role: Create feature_list.json
	if strings.Contains(prompt, "Analyze the following application specification") ||
	   strings.Contains(prompt, "Create a JSON feature list") {
		return `
```bash
cat << 'EOF' > feature_list.json
{
  "project_name": "primes",
  "features": [
    {
      "id": "req-primes",
      "category": "functional",
      "priority": "critical",
      "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000",
      "status": "pending",
      "passes": false,
      "steps": null,
      "dependencies": null
    }
  ]
}
EOF
agent-bridge import feature_list.json
```
`, nil
	}

	// 2. Coding Agent: Implement Primes
	// Triggered by "Implement a python script named 'primes.py'" or similar from feature list
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "[PRIMES]") {
		return `
```bash
cat << 'EOF' > primes.py
import json

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
        json.dump({"primes": primes}, f)

if __name__ == "__main__":
    main()
EOF

# Create test
cat << 'EOF' > test_primes.py
import json
import unittest
from primes import calculate_primes

class TestPrimes(unittest.TestCase):
    def test_count(self):
        primes = calculate_primes(10000)
        self.assertEqual(len(primes), 1229)

if __name__ == "__main__":
    unittest.main()
EOF

# Run it
python3 primes.py
python3 test_primes.py

# Signal Done
agent-bridge feature set req-primes --status done --passes true

# Commit (check status first to avoid empty commit failure loop)
git add .
git diff --cached --quiet || git commit -m "Implement primes.py"
```
`, nil
	}

	// 3. QA Agent
	if strings.Contains(prompt, "Run the following quality assurance checks") || strings.Contains(prompt, "QA_PASSED") {
		return `
```bash
echo "Running QA checks..."
python3 test_primes.py
agent-bridge signal QA_PASSED true
```
`, nil
	}

	// 4. Manager Agent
	if strings.Contains(prompt, "Review the following QA report") || strings.Contains(prompt, "PROJECT_SIGNED_OFF") {
		return `
```bash
echo "Manager approving project..."
agent-bridge signal --privileged PROJECT_SIGNED_OFF true
```
`, nil
	}

	// Default Fallback
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
