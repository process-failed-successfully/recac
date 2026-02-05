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
	// Debug logging to help trace mock agent behavior in CI
	fmt.Printf("[MockAgent] Received Prompt: %s...\n", truncateString(prompt, 50))

	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// 1. QA Agent Heuristic
	// Detects if the agent is acting as QA to approve the project
	if strings.Contains(prompt, "YOUR ROLE - QA AGENT") {
		return `
I have verified the project and it looks good.

` + "```bash" + `
agent-bridge signal QA_PASSED true
` + "```" + `
`, nil
	}

	// 2. Manager Agent Heuristic
	// Detects if the agent is the Manager receiving a report to sign off
	if strings.Contains(prompt, "Manager Agent") || (strings.Contains(prompt, "YOUR ROLE - PROJECT MANAGER")) {
		return `
I approve the project.

` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true
` + "```" + `
`, nil
	}

	// 3. TPM (Ticket Generation) Heuristic
	// Returns a valid JSON plan for the prime-python scenario
	if isTPMPrompt(prompt) {
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Implement a Python script that generates prime numbers.\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Story",
    "acceptance_criteria": [
      "Create primes.py",
      "Implement verify_prime function",
      "Add unit tests"
    ],
    "children": []
  }
]`, nil
	}

	// 4. Initializer Heuristic
	// If it's the initializer, we return a simple acknowledgement.
	// We check for "INITIALIZER" but explicitly NOT "implementation" to avoid overlap if prompt leaks context.
	if strings.Contains(prompt, "INITIALIZER") {
		return "I have analyzed the spec and created the plan.", nil
	}

	// 5. Prime Python Heuristic
	// Detects the prime-python scenario and generates the correct script
	if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py") {
		return `
I will implement the prime number generator.

` + "```bash" + `
# Create the python script
# Using EOF without quotes to allow variable expansion if needed, but here simple cat is fine.
# Added error handling for python execution
cat <<EOF > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(10000) if is_prime(x)]
with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)
EOF

# Run the script to generate the JSON, fail if it fails
python3 primes.py || { echo "Python script execution failed"; exit 1; }

# Ensure git configuration
if [ -n "$GITHUB_TOKEN" ]; then
  git config --global url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"
fi
git config --global user.email "agent@recac.io"
git config --global user.name "RECAC Agent"

# Add and push files
git add primes.py primes.json
git commit -m "Add primes.py and primes.json"
# Explicitly push to the current branch on origin to ensure the remote is updated
# We strip 'refs/heads/' to get the branch name if needed, or just push HEAD
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
git push origin HEAD:refs/heads/$CURRENT_BRANCH

# Mark features as done
agent-bridge feature list --json | jq -r '.features[].id' | xargs -I {} agent-bridge feature set {} --status done --passes true

# Signal completion
echo "COMPLETED"
` + "```" + `
`, nil
	}

	// 6. Implementation (Default) Heuristic
	// This is the critical part for preventing NO-OP loops.
	// We MUST return bash commands that:
	// a) Do some work (create files)
	// b) Update feature status via agent-bridge so the loop progresses
	// c) Signal completion

	return `
I will implement the requested features.

` + "```bash" + `
# Create the file
echo "def is_prime(n): return n > 1" > primes.py

# Mark features as done
# Dynamic discovery to handle injected features
agent-bridge feature list --json | jq -r '.features[].id' | xargs -I {} agent-bridge feature set {} --status done --passes true

# Signal completion to break the loop
echo "COMPLETED"
` + "```" + `
`, nil
}

func isTPMPrompt(prompt string) bool {
	p := truncateString(prompt, 200) // Optimization
	return (containsIgnoreCase(p, "Technical Program Manager") || containsIgnoreCase(p, "TPM"))
}

func containsIgnoreCase(s, substr string) bool {
	s, substr = strings.ToLower(s), strings.ToLower(substr)
	return strings.Contains(s, substr)
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
