package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// MockAgent is a simple mock agent for testing and mock mode
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

var pendingStatusRegex = regexp.MustCompile(`"status":\s*"pending"`)

// Send implements the Agent interface
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristics for E2E Smoke Tests

	// 1. Planning Phase (Technical Program Manager)
	if strings.Contains(prompt, "Technical Program Manager") && strings.Contains(prompt, "break down") {
		return `[
  {
    "id": "req-primes",
    "type": "task",
    "title": "Implement Prime Number Generator",
    "description": "Create a python script to generate prime numbers.",
    "dependencies": []
  }
]`, nil
	}

	// 2. Architecture Phase (Lead Software Architect)
	// Or simply "break down" for feature list generation if TPM didn't cover it?
	// Usually Architect generates feature_list.json
	if strings.Contains(prompt, "Lead Software Architect") || (strings.Contains(prompt, "feature_list.json") && strings.Contains(prompt, "break down")) {
		return `#!/bin/bash
cat <<EOF > feature_list.json
{
  "features": [
    {
      "id": "task-primes",
      "status": "pending",
      "passes": false,
      "dependencies": {}
    }
  ]
}
EOF
echo "Generated feature_list.json"
`, nil
	}

	// 3. QA Agent
	if strings.Contains(prompt, "QA AGENT") {
		return `#!/bin/bash
echo "Running QA checks..."
# Verify primes.py exists
if [ -f primes.py ]; then
    echo "primes.py found."
    python3 -c "import primes" || echo "primes module valid"
fi
agent-bridge signal QA_PASSED true || echo "Signal sent"
`, nil
	}

	// 4. Manager Agent (Sign off)
	if strings.Contains(prompt, "Manager Agent") {
		return `#!/bin/bash
echo "Manager signing off."
agent-bridge signal PROJECT_SIGNED_OFF true || echo "Signal sent"
`, nil
	}

	// 5. Execution Phase: Specific Task (Primes)
	// Check for ID:[PRIMES] or explicit instruction.
	// IMPORTANT: Ensure we are NOT in the planning phase (Technical Program Manager) or other roles
	// which might mention "primes.py" in the context but expect JSON.
	isPlanning := strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "Lead Software Architect")
	if !isPlanning && (strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "Implement prime calculation logic in primes.py") || strings.Contains(prompt, "primes.py")) {
		return `#!/bin/bash
# Create primes.py
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

def generate_primes(n):
    primes = []
    for i in range(2, n + 1):
        if is_prime(i):
            primes.append(i)
    return primes

if __name__ == "__main__":
    import json
    print(json.dumps(generate_primes(50)))
EOF

# Create primes.json output
python3 primes.py > primes.json

# Update status
agent-bridge feature set task-primes status done || echo "Feature updated"
git config user.email "mock@example.com" || true
git config user.name "Mock Agent" || true
git add primes.py primes.json || echo "Added files"
git commit -m "Implement primes" || echo "Committed"
echo "Success: Mock command executed"
`, nil
	}

	// 6. Generic Execution Phase (Task Completion)
	// If we see "status": "pending" in the prompt (context), assume we need to finish a task.
	if pendingStatusRegex.MatchString(prompt) {
		return `#!/bin/bash
echo "Task completed"
# Attempt to mark current task as done if ID is inferable?
# For mock, we just say done.
# But we need to update feature_list.json or DB.
# Assuming standard file-based workflow for mock:
if [ -f feature_list.json ]; then
    sed -i 's/"status": "pending"/"status": "done"/g' feature_list.json
fi
`, nil
	}

	// Default fallback
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response.\nPrompt preview: %s...",
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
