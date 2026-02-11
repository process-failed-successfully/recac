package agent

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// MockAgent is a smart mock agent for testing and E2E scenarios
// It implements heuristics to simulate different agent roles without using an LLM.
type MockAgent struct {
	responsePrefix string
	forcedResponse string
}

// NewMockAgent creates a new mock agent
func NewMockAgent() *MockAgent {
	return &MockAgent{
		responsePrefix: "Mock Agent",
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

	// 1. TPM Role (Planning)
	// Heuristic: "Technical Program Manager" in prompt
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") {
		// Return JSON plan (Array of tickets)
		return `[
  {
    "title": "Implement Primes Script",
    "description": "Create a python script that calculates prime numbers.",
    "type": "task",
    "status": "todo"
  }
]`, nil
	}

	// 2. Initializer Agent
	// Heuristic: "INITIALIZER AGENT" header
	if strings.Contains(prompt, "INITIALIZER AGENT") {
		// Write feature_list.json to disk
		// We skip 'agent-bridge import' to avoid binary dependency issues in some CI envs,
		// relying on runner.Session fallback to read the file.
		return "```bash\n" + `
echo '{"project_name": "PRIMES", "features": [{"description": "Implement Primes Script", "status": "pending"}]}' > feature_list.json
echo "Initialized feature_list.json"
` + "\n```", nil
	}

	// 3. Coding Agent (Primes Implementation)
	// Heuristic: "CODING AGENT" or task specific keywords
	if strings.Contains(prompt, "CODING AGENT") || strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "Prime Number Script") {
		// Return bash script to implement primes.py, commit, push, and signal completion
		// We use RECAC_PROJECT_ID for branch name to ensure consistency with E2E verification
		script := "```bash\n" + `
# Create the script
cat << 'EOF' > primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

import sys
if __name__ == "__main__":
    if len(sys.argv) > 1:
        n = int(sys.argv[1])
        print(f"{n} is prime: {is_prime(n)}")
    else:
        print("Primes check passed")
EOF

# Verify it works
python3 primes.py

# Git operations
git config --global user.email "mock@example.com"
git config --global user.name "Mock Agent"
BRANCH_NAME="agent/${RECAC_PROJECT_ID:-PRIMES-mock}"
git checkout -b "$BRANCH_NAME" || git checkout "$BRANCH_NAME"
git add primes.py
git commit -m "Implement primes.py" || true
# We use a token if available, or assume SSH/auth is set up
git push origin "$BRANCH_NAME" || echo "Push failed (expected in local mock)"

# Update status and signal
agent-bridge feature update "Implement Primes Script" --status completed || true
agent-bridge signal QA_PASSED true || touch QA_PASSED
` + "\n```"
		return script, nil
	}

	// 4. QA Agent
	if strings.Contains(prompt, "QA AGENT") {
		return "```bash\n" + `
echo "QA Passed"
agent-bridge signal QA_PASSED true || touch QA_PASSED
` + "\n```", nil
	}

	// 5. Project Manager
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return "```bash\n" + `
echo "Project Signed Off"
agent-bridge signal PROJECT_SIGNED_OFF true || touch PROJECT_SIGNED_OFF
` + "\n```", nil
	}

	// Default fallback for generic prompts
	return fmt.Sprintf("%s received: %s...", m.responsePrefix, truncateString(prompt, 50)), nil
}

// SendStream implements the Agent interface
func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := m.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		// Minimal delay to prevent timeouts in tests while simulating async
		time.Sleep(10 * time.Millisecond)
		onChunk(resp)
	}
	return resp, err
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
