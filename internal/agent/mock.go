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
// It uses heuristic matching to simulate an autonomous agent for E2E tests
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// 1. TPM Agent (Project Manager) - Detects ticket spec
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "## YOUR ROLE - PROJECT MANAGER") {
		return `[
  {
    "id": "req-primes-script",
    "category": "functional",
    "priority": "critical",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000",
    "status": "pending",
    "passes": false
  },
  {
    "id": "req-primes-output",
    "category": "functional",
    "priority": "critical",
    "description": "Output results to primes.json",
    "status": "pending",
    "passes": false
  },
  {
    "id": "req-validate-output",
    "category": "functional",
    "priority": "critical",
    "description": "Validate that the output file contains a 'primes' list",
    "status": "pending",
    "passes": false
  },
  {
    "id": "req-verify-count",
    "category": "functional",
    "priority": "critical",
    "description": "Verify that exactly 1229 primes are calculated",
    "status": "pending",
    "passes": false
  },
  {
    "id": "req-commit-file",
    "category": "functional",
    "priority": "critical",
    "description": "Commit primes.json to the repository",
    "status": "pending",
    "passes": false
  }
]`, nil
	}

	// 2. Initializer Agent - Bootstraps the workspace
	// Uses explicit file writing to ensure state persistence even if agent-bridge fails or DB is mocked differently
	if strings.Contains(prompt, "Initializer Agent") || strings.Contains(prompt, "## YOUR ROLE - INITIALIZER AGENT") {
		return "```bash\n" + `
# Write features directly to file for robustness
cat << 'EOF' > feature_list.json
{
  "features": [
    {
      "id": "req-primes-script",
      "category": "functional",
      "priority": "critical",
      "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000"
    },
    {
      "id": "req-primes-output",
      "category": "functional",
      "priority": "critical",
      "description": "Output results to primes.json"
    },
    {
      "id": "req-validate-output",
      "category": "functional",
      "priority": "critical",
      "description": "Validate that the output file contains a 'primes' list"
    },
    {
      "id": "req-verify-count",
      "category": "functional",
      "priority": "critical",
      "description": "Verify that exactly 1229 primes are calculated"
    },
    {
      "id": "req-commit-file",
      "category": "functional",
      "priority": "critical",
      "description": "Commit primes.json to the repository"
    }
  ]
}
EOF

# Also try to import via bridge if available, but ignore errors
if command -v agent-bridge > /dev/null; then
  cat feature_list.json | agent-bridge import || true
fi
` + "\n```", nil
	}

	// 3. Coding Agent (Primes Scenario) - Implements the solution
	if strings.Contains(prompt, "YOUR ROLE - CODING AGENT") || (strings.Contains(strings.ToUpper(prompt), "PRIME") && strings.Contains(strings.ToUpper(prompt), "PYTHON")) {
		return `I will implement the prime number calculation script.

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
output = {"primes": primes}

with open("primes.json", "w") as f:
    json.dump(output, f)
EOF

# Run it to generate the JSON
python3 primes.py

# Verify count
count=$(python3 -c "import json; print(len(json.load(open('primes.json'))['primes']))")
if [ "$count" -eq "1229" ]; then
    echo "Verification Passed: 1229 primes found."
else
    echo "Verification Failed: $count primes found."
    exit 1
fi

# Commit results
git add primes.py primes.json
git commit -m "Add primes.py and primes.json" || echo "Nothing to commit"

# Update features via bridge if available
if command -v agent-bridge > /dev/null; then
    agent-bridge feature set req-primes-script --status done --passes true
    agent-bridge feature set req-primes-output --status done --passes true
    agent-bridge feature set req-validate-output --status done --passes true
    agent-bridge feature set req-verify-count --status done --passes true
    agent-bridge feature set req-commit-file --status done --passes true

    # Signal completion
    agent-bridge signal COMPLETED true
fi
` + "```", nil
	}

	// 4. QA Agent / Manager Review - Signs off
	if strings.Contains(strings.ToUpper(prompt), "QA") || strings.Contains(strings.ToUpper(prompt), "REVIEW") || strings.Contains(strings.ToUpper(prompt), "VERIFY") {
		return `The implementation looks correct.
` + "```bash" + `
if command -v agent-bridge > /dev/null; then
    agent-bridge signal PROJECT_SIGNED_OFF true
fi
` + "```", nil
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
