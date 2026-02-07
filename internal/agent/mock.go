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

	// Heuristics for E2E Smoke Test (Prime Python Scenario)

	// 0. Initializer - Feature List Generation
	// Trigger: "INITIALIZER" in prompt
	if strings.Contains(strings.ToUpper(prompt), "INITIALIZER") {
		// If it's the Primes scenario (detected by keywords or ticket ID)
		if strings.Contains(prompt, "PRIMES") || strings.Contains(prompt, "MFLP-7282") {
			return `{
  "project_name": "MFLP-7282",
  "features": [
    {
      "id": "req-the-script-primes-py-is-implem",
      "description": "The script primes.py is implemented",
      "status": "todo",
      "priority": "1"
    },
    {
      "id": "req-the-output-is-written-to-a-fil",
      "description": "The output is written to a file named primes.json",
      "status": "todo",
      "priority": "1"
    },
    {
      "id": "req-the-primes-json-file-contains-",
      "description": "The primes.json file contains a key 'primes'",
      "status": "todo",
      "priority": "1"
    },
    {
      "id": "req-the-list-of-primes-in-primes-j",
      "description": "The list of primes in primes.json is correct for n=10000",
      "status": "todo",
      "priority": "1"
    }
  ]
}`, nil
		}
	}

	// 1. Project Manager - Ticket Generation
	// Trigger: "ROLE - TECHNICAL PROGRAM MANAGER" or similar, AND "PRIMES"
	// We check for "Technical Program Manager" or "TPM" to be robust.
	isTPM := strings.Contains(prompt, "TECHNICAL PROGRAM MANAGER") ||
		strings.Contains(prompt, "Technical Program Manager") ||
		strings.Contains(prompt, "project management") ||
		strings.Contains(prompt, "TPM")

	if isTPM && strings.Contains(prompt, "PRIMES") && strings.Contains(prompt, "JSON") {
		return `[
  {
    "title": "ID:[PRIMES] Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'. The output file MUST be named 'primes.json' and contain a single key 'primes' with the list of integers.",
    "type": "Task",
    "priority": "Critical"
  }
]`, nil
	}

	// 2. Coding Agent - Implementation
	// Trigger: "ROLE - CODING AGENT" or similar, AND "primes.py"
	// We also check if we are being asked to implement it, vs just reviewing.
	isDeveloper := strings.Contains(prompt, "CODING AGENT") ||
		strings.Contains(prompt, "Coding Agent") ||
		strings.Contains(prompt, "Developer") ||
		strings.Contains(prompt, "Software Engineer")

	if isDeveloper &&
		strings.Contains(prompt, "primes.py") &&
		!strings.Contains(prompt, "Review") {

		// Check if it's already implemented to avoid infinite loops
		// If the prompt contains "current state" and "primes.py", we might assume it's done?
		// But for the smoke test, we just want to output the solution once.
		// The runner usually sends the feature list.

		return `I will implement the primes script and verify it.

` + "```bash" + `
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
    primes = calculate_primes(10000)
    with open('primes.json', 'w') as f:
        json.dump({'primes': primes}, f)

if __name__ == "__main__":
    main()
EOF

# Run the script
python3 primes.py

# Verify output file exists
if [ -f primes.json ]; then
  echo "Output file created."
  # Mark relevant features as done
  agent-bridge feature set req-the-script-primes-py-is-implem --status done --passes true
  agent-bridge feature set req-the-output-is-written-to-a-fil --status done --passes true
fi

# Verify content
if cat primes.json | grep -q "primes"; then
    echo "JSON content verified."
    agent-bridge feature set req-the-primes-json-file-contains- --status done --passes true
    agent-bridge feature set req-the-list-of-primes-in-primes-j --status done --passes true
fi

# Commit
git add primes.py primes.json
git commit -m "Implement primes.py"
` + "```" + `
`, nil
	}

	// 3. Fallback / Review
	if strings.Contains(prompt, "Review") || strings.Contains(prompt, "QA") {
		return "The implementation looks correct and passes all checks.", nil
	}

	// Default response
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
