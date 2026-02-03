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

	// Smart Mock Logic for Smoke Tests
	// 1. Ticket Generation Request (Prime Python Scenario)
	if strings.Contains(prompt, "ID:[PRIMES]") && strings.Contains(prompt, "JSON format") {
		return `[
  {
    "title": "[GEN] Create Prime Number Script",
    "description": "Create a python script named 'primes.py' that calculates primes < 10000 and outputs to 'primes.json'. ID:[PRIMES]",
    "type": "Task",
    "children": []
  }
]`, nil
	}

	// 2. Initializer Logic
	// Matches prompt from Initializer Agent asking to create feature list
	// We use Case Insensitive matching and check for "INITIALIZER" to be robust.
	upperPrompt := strings.ToUpper(prompt)
	if strings.Contains(upperPrompt, "INITIALIZER") || strings.Contains(prompt, "feature_list.json") {
		// Debug logging to help identify why this branch is taken (or not)
		fmt.Println("[MockAgent] Matched Initializer Logic")
		return `I will create the initial feature list based on the requirements.

` + "```bash" + `
cat << 'EOF' > feature_list.json
[
  {
    "id": "req-primes",
    "name": "Implement Prime Number Script",
    "description": "Create a python script to calculate primes.",
    "status": "pending",
    "files": ["primes.py"]
  }
]
EOF

# Verify agent-bridge is available (for debug)
if command -v agent-bridge > /dev/null; then
  echo "agent-bridge available"
fi
` + "```" + `
`, nil
	}

	// 3. Implementation Request (Writing the file)
	// Matches prompt asking to implement "PRIMES" or "primes.py"
	// Also check for [GEN] tag which appears in E2E tests
	if strings.Contains(prompt, "PRIMES") || strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "[GEN]") || strings.Contains(upperPrompt, "PRIME") {
		return `I will create the primes.py script and the json output as requested.

` + "```bash" + `
# Create the python script
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

primes_list = get_primes(10000)
with open("primes.json", "w") as f:
    json.dump({"primes": primes_list}, f)
EOF

# Run the script to generate the json
python3 primes.py

# Add and commit
git add primes.py primes.json
git commit -m "Add primes script and output"

# Update feature status to Done
# We use $(agent-bridge feature list --json | jq -r '.features[0].id') to get the ID dynamically if needed,
# but for smoke tests we know the ID is likely 'req-primes' or 'req-primes-json-contains-correct-primes'
# Let's try to be robust and set 'req-primes' (from Initializer) AND 'req-primes-json-contains-correct-primes' (from Environment Injection if any)
agent-bridge feature set req-primes --status done --passes true
agent-bridge feature set req-primes-json-contains-correct-primes --status done --passes true
` + "```" + `
`, nil
	}

	// 4. Nothing to Commit (Completion Guard)
	// If the agent sees "nothing to commit", it implies the coding task is done.
	// We signal COMPLETED to trigger QA.
	if strings.Contains(prompt, "nothing to commit") || strings.Contains(prompt, "working tree clean") {
		return `It seems the work is complete and there are no more changes to commit.

` + "```bash" + `
agent-bridge signal COMPLETED true
` + "```" + `
`, nil
	}

	// 5. QA Agent Role
	if strings.Contains(upperPrompt, "QA AGENT") {
		return `QA verification passed.

` + "```bash" + `
agent-bridge signal QA_PASSED true
` + "```" + `
`, nil
	}

	// 6. Project Manager Role
	if strings.Contains(upperPrompt, "PROJECT MANAGER") {
		return `Project approved.

` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true
` + "```" + `
`, nil
	}

	// Default Mock Response
	// We include a no-op bash block to ensure the executor doesn't trip the "no commands" circuit breaker
	// We strip backticks from the preview to avoid confusing the regex parser
	fmt.Println("[MockAgent] Fallback to default response (No matcher triggered)")
	preview := strings.ReplaceAll(truncateString(prompt, 100), "`", "'")
	// Ensure regex matching by surrounding content with whitespace inside delimiters
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...\n\n```bash\n# no-op to prevent circuit breaker\necho 'mock agent alive'\n```\n",
		m.responsePrefix, len(prompt), preview)
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
