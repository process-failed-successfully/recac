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

	lowerPrompt := strings.ToLower(prompt)

	// 1. Generate From Spec (CLI Command)
	// Must check this first to avoid confusion with Initializer if prompt contains both
	if strings.Contains(lowerPrompt, "generate-from-spec") || strings.Contains(lowerPrompt, "technical program manager") {
		// Explicitly exclude Initializer triggers to prevent false positives
		if !strings.Contains(lowerPrompt, "feature list") && !strings.Contains(lowerPrompt, "extract") {
			return `[
  {
    "ID": "PRIMES",
    "Summary": "Create Prime Number Script",
    "Desc": "Create a python script named 'primes.py' that calculates primes < 10000.",
    "Type": "Task"
  }
]`, nil
		}
	}

	// 2. Initializer (Feature List)
	if strings.Contains(lowerPrompt, "initialize") || strings.Contains(lowerPrompt, "feature_list.json") {
		return `I will create the feature list.

` + "```bash" + `
# Create feature list
cat << 'EOF' > feature_list.json
{
  "project_name": "mock-project",
  "features": [
    {
      "id": "req-primes",
      "description": "Implement primes.py",
      "status": "todo"
    }
  ]
}
EOF

# Signal completion if bridge exists
if command -v agent-bridge >/dev/null 2>&1; then
    agent-bridge signal INITIALIZED true
fi
` + "```", nil
	}

	// 3. QA Agent
	if strings.Contains(prompt, "QA AGENT") {
		return `I have verified the features.

` + "```bash" + `
if command -v agent-bridge >/dev/null 2>&1; then
    agent-bridge signal QA_PASSED true
fi
` + "```", nil
	}

	// 4. Project Manager
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return `I approve the work.

` + "```bash" + `
if command -v agent-bridge >/dev/null 2>&1; then
    agent-bridge signal PROJECT_SIGNED_OFF true
fi
` + "```", nil
	}

	// 5. Primes Scenario (Implementation)
	if strings.Contains(lowerPrompt, "req-primes") || strings.Contains(lowerPrompt, "primes.py") || strings.Contains(prompt, "[PRIMES]") {
		// Infinite loop breaker
		if strings.Contains(lowerPrompt, "nothing to commit") || strings.Contains(lowerPrompt, "working tree clean") {
			return `Work is complete.

` + "```bash" + `
if command -v agent-bridge >/dev/null 2>&1; then
    agent-bridge update req-primes --status done
    agent-bridge signal COMPLETED true
fi
` + "```", nil
		}

		return `I will implement the primes script.

` + "```bash" + `
# Configure git
git config user.email "mock@agent.com"
git config user.name "Mock Agent"

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
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run it
python3 primes.py

# Commit
git add primes.py primes.json
git commit -m "Implement primes.py" || echo "Nothing to commit"

# Signal progress
if command -v agent-bridge >/dev/null 2>&1; then
    agent-bridge update req-primes --status done
    # Also update injected features if any
    agent-bridge update req-primes-py-exists --status done
    agent-bridge update req-primes-json-contains-correct-p --status done
fi
` + "```", nil
	}

	// Default Fallback
	// Put # no-op at start to ensure it is matched as a bash block if regex is loose
	// and to ensure non-empty response.
	// Use single quotes for prompt preview to avoid breaking markdown.
	safePrompt := strings.ReplaceAll(truncateString(prompt, 100), "`", "'")

	response := fmt.Sprintf(`I received your prompt.

`+"```bash"+`
# no-op
echo "Mock Agent received %d chars"
`+"```"+`

Prompt preview: %s...`, len(prompt), safePrompt)

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
