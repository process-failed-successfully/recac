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

	upperPrompt := strings.ToUpper(prompt)

	// --- Heuristics for E2E Smoke Test (prime-python) ---

	// 1. Initializer: "Analyze the following application specification" or similar
	if strings.Contains(upperPrompt, "ANALYZE") && strings.Contains(upperPrompt, "SPECIFICATION") {
		return `I have analyzed the specification. Here is the feature breakdown:

` + "```bash" + `
cat << 'EOF' > feature_list.json
{
  "project_name": "recac-jira-e2e",
  "features": [
    {
      "id": "PRIMES",
      "description": "Create Prime Number Script",
      "priority": "MVP",
      "status": "todo",
      "passes": false,
      "steps": [
        "Create primes.py",
        "Generate primes.json",
        "Verify output"
      ]
    }
  ]
}
EOF
` + "```" + `
`, nil
	}

	// 2. Coding Agent: Working on PRIMES task
	// Prompt usually contains "PRIMES" or "primes.py"
	if strings.Contains(upperPrompt, "PRIMES") || strings.Contains(upperPrompt, "PRIMES.PY") {
		return `I will implement the prime number script.

` + "```bash" + `
# 1. Create primes.py script
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(10000) if is_prime(x)]
with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)

print(f"Generated {len(primes)} primes.")
EOF

# 2. Generate the output file
python3 primes.py

# 3. Commit the changes
git add primes.py primes.json
git commit -m "Add primes script and output"
git push origin HEAD

# 4. Mark task as done in feature_list.json (for local tracking)
# In real scenario, we might update DB, but for file-based tracking:
cat << 'EOF' > feature_list.json
{
  "project_name": "recac-jira-e2e",
  "features": [
    {
      "id": "PRIMES",
      "description": "Create Prime Number Script",
      "priority": "MVP",
      "status": "done",
      "passes": true,
      "steps": [
        "Create primes.py",
        "Generate primes.json",
        "Verify output"
      ]
    }
  ]
}
EOF
` + "```" + `
`, nil
	}

	// 3. QA Agent: Verify or QA
	if strings.Contains(upperPrompt, "QA") || strings.Contains(upperPrompt, "VERIFY") {
		return `QA_PASSED

The code looks correct and meets the requirements.
`, nil
	}

	// 4. Manager: Review or Plan
	// If the manager asks for status or plan, and we see everything is done (or generic), we can sign off.
	if strings.Contains(upperPrompt, "MANAGER") || strings.Contains(upperPrompt, "REVIEW") || strings.Contains(upperPrompt, "PLAN") {
		return `The project seems complete.

` + "```bash" + `
agent-bridge signal --privileged PROJECT_SIGNED_OFF true
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
