package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// MockAgent is a smart mock agent for testing and mock mode
// It returns predefined responses based on the prompt content
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

	// 1. TPM Agent (Planning Phase)
	// Trigger: "technical program manager" or "break down this spec"
	if strings.Contains(strings.ToLower(prompt), "technical program manager") || strings.Contains(strings.ToLower(prompt), "id:[primes]") {
		return m.handlePlanningPhase(prompt), nil
	}

	// 2. Coding Agent (Implementation Phase)
	// Trigger: "write" or "implement" AND "bash" (usually asks for bash blocks)
	// Or specific task IDs
	if strings.Contains(strings.ToLower(prompt), "implement") || strings.Contains(strings.ToLower(prompt), "create") || strings.Contains(prompt, "primes.py") {
		return m.handleCodingPhase(prompt), nil
	}

	// 3. Testing/QA Phase
	// If output contains success indicators, we are done.
	if strings.Contains(prompt, "PASS") || strings.Contains(prompt, "ok") || strings.Contains(prompt, "generated 1229 primes") {
		return "Task completed. All tests passed.", nil
	}

	// Default fallback
	return fmt.Sprintf("%s: I received your prompt (%d chars).", m.responsePrefix, len(prompt)), nil
}

// SendStream implements the Agent interface
func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := m.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(resp)
	}
	return resp, err
}

func (m *MockAgent) handlePlanningPhase(prompt string) string {
	// Extract Repo URL from prompt to ensure consistency
	repoRegex := regexp.MustCompile(`(?i)Repo: (https?://\S+)`)
	repoURL := "https://github.com/process-failed-successfully/recac-jira-e2e" // Default
	matches := repoRegex.FindStringSubmatch(prompt)
	if len(matches) > 1 {
		repoURL = strings.Trim(matches[1], "`")
	}

	// Return a valid JSON ticket plan
	// We hardcode the structure for the 'primes' scenario which is commonly used in E2E
	return fmt.Sprintf(`
[
  {
    "title": "ID:[PRIMES] Prime Number Generator",
    "description": "Implement a prime number generator script.\n\nREQUIRED FEATURES:\n- Calculate primes < 10000\n- Output to primes.json\n\nRepo: %s",
    "type": "Task",
    "blocked_by": [],
    "acceptance_criteria": [
      "primes.py exists",
      "primes.json contains 1229 primes"
    ],
    "children": []
  }
]
`, repoURL)
}

func (m *MockAgent) handleCodingPhase(prompt string) string {
	// Return a Bash script to implement the feature
	return `
Here is the implementation for the prime number generator.

` + "```bash" + `
# Create primes.py
cat << 'EOF' > primes.py
import json

def get_primes(n):
    primes = []
    for num in range(2, n):
        is_prime = True
        for i in range(2, int(num ** 0.5) + 1):
            if num % i == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(num)
    return primes

primes = get_primes(10000)
print(f"Generated {len(primes)} primes")

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run it
python3 primes.py

# Commit
git add primes.py primes.json
git commit -m "Implement prime number generator"
git push origin HEAD
` + "```" + `
`
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
