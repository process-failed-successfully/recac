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

	// Heuristics for E2E Smoke Test (Mock Mode)

	// 1. Initializer: Create feature_list.json
	if strings.Contains(prompt, "CREATE FEATURE_LIST.JSON") {
		return `
I will create the feature list.

` + "```bash" + `
cat <<EOF > feature_list.json
{
  "project_name": "prime-python",
  "features": [
    {
      "id": "req-git-initialized",
      "description": "Git repository initialized and remote set",
      "completed": false
    },
    {
      "id": "req-primes-py-exists",
      "description": "primes.py script created and functioning",
      "completed": false
    },
    {
      "id": "req-primes-json-exists",
      "description": "primes.json output file generated and valid",
      "completed": false
    }
  ]
}
EOF

# Import features
agent-bridge import < feature_list.json || echo "Import failed but continuing"
` + "```" + `
`, nil
	}

	// 2. Technical Program Manager (TPM): Generate Ticket JSON
	if strings.Contains(prompt, "Technical Program Manager") {
		return `
[
  {
    "id": "PRIMES",
    "type": "Task",
    "summary": "Create Prime Number Script",
    "description": "Implement primes.py to calculate primes < 10000",
    "acceptance_criteria": [
        "primes.py exists",
        "primes.json generated"
    ]
  }
]
`, nil
	}

	// 3. Project Manager: Sign off
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return `
I confirm the work is complete.

` + "```bash" + `
agent-bridge signal COMPLETED true --privileged || echo "Signal failed"
` + "```" + `
`, nil
	}

	// 4. QA Agent: Run tests
	if strings.Contains(prompt, "QA AGENT") {
		return `
I will verify the implementation.

` + "```bash" + `
python3 primes.py || echo "Execution failed"
if [ -f primes.json ]; then
    echo "primes.json exists"
    agent-bridge signal QA_PASSED true --privileged || echo "Signal failed"
else
    echo "primes.json missing"
fi
` + "```" + `
`, nil
	}

	// 5. Coding Agent (Primes Scenario)
	// We check for specific feature IDs or keywords
	if strings.Contains(prompt, "CODING AGENT") || strings.Contains(prompt, "primes.py") {
		return `
I will implement the prime number script.

` + "```bash" + `
# Initialize git if needed
git init || true
git config user.email "mock@agent.com" || true
git config user.name "Mock Agent" || true

# Create primes.py
cat <<EOF > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [i for i in range(10000) if is_prime(i)]

with open('primes.json', 'w') as f:
    json.dump({'primes': primes}, f)
EOF

# Run it to generate output
python3 primes.py || echo "Script execution failed"

# Commit results
git add primes.py primes.json || true
git commit -m "Implement primes.py" || echo "Nothing to commit"

# Mark features as complete
agent-bridge feature set req-git-initialized true || true
agent-bridge feature set req-primes-py-exists true || true
agent-bridge feature set req-primes-json-exists true || true
` + "```" + `
`, nil
	}

	// Default fallback
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
