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

	// 1. Project Manager - Ticket Generation
	// Trigger: "ROLE - TECHNICAL PROGRAM MANAGER" or similar, AND "PRIMES"
	upperPrompt := strings.ToUpper(prompt)
	if (strings.Contains(upperPrompt, "TECHNICAL PROGRAM MANAGER") || strings.Contains(upperPrompt, "TPM") || strings.Contains(upperPrompt, "PROJECT MANAGEMENT")) &&
		strings.Contains(upperPrompt, "PRIMES") && strings.Contains(upperPrompt, "JSON") {
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
	// We allow "Review" in the prompt because the template includes instructions like "SELF-REVIEW" and "Manager Review".
	if (strings.Contains(prompt, "ROLE - CODING AGENT") || strings.Contains(prompt, "Developer")) &&
		strings.Contains(prompt, "primes.py") {

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

	// 3. Initializer Agent - Feature List Creation
	// Trigger: "ROLE - INITIALIZER AGENT" (case-insensitive)
	if strings.Contains(strings.ToUpper(prompt), "ROLE - INITIALIZER AGENT") {
		// Return a valid feature list import command
		return `I will initialize the project with the required features.

` + "```bash" + `
cat << 'EOF' | agent-bridge import
{
  "project_name": "Mock Project",
  "features": [
    {
      "id": "req-initial-setup",
      "category": "functional",
      "priority": "MVP",
      "description": "Initial project setup and feature list creation",
      "status": "pending",
      "steps": ["Verify feature_list.json exists"],
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
echo "Feature list initialized."
` + "```" + `
`, nil
	}

	// 4. Manager / Review Agent
	// Trigger: "ROLE - PROJECT MANAGER" or similar
	// Case insensitive check for MANAGER or PROJECT MANAGER, while excluding Developer/Coding Agent roles
	if (strings.Contains(upperPrompt, "MANAGER") || strings.Contains(upperPrompt, "PROJECT MANAGER")) &&
		!strings.Contains(upperPrompt, "CODING AGENT") &&
		!strings.Contains(upperPrompt, "DEVELOPER") {

		// If it's a Manager review, and features seem done (implied by getting here in smoke test),
		// we should signal project completion.
		return `I have reviewed the project status. All features appear to be implemented and verified.

` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true
echo "Project signed off by Manager."
` + "```" + `
`, nil
	}

	// 5. QA / Review Fallback
	if (strings.Contains(prompt, "Review") || strings.Contains(prompt, "QA")) &&
		!strings.Contains(prompt, "CODING AGENT") &&
		!strings.Contains(prompt, "Developer") {
		return "The implementation looks correct and passes all checks.", nil
	}

	// Default response
	// We include a harmless command to prevent 'NO-OP LOOP' errors in the runner's circuit breaker
	response := fmt.Sprintf(`%s:

I received your prompt (%d characters). In mock mode, I would process this request and provide a response.

Prompt preview: %s...

`+"```bash"+`
echo "Mock Agent Default Response"
`+"```"+`
`, m.responsePrefix, len(prompt), truncateString(prompt, 100))
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
