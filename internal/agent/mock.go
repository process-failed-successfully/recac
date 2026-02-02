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

	// 1. Completion Check (Clean Working Tree)
	// If the prompt indicates that git has nothing to commit, we should signal completion
	// to prevent infinite no-op loops in smoke tests.
	// We check this FIRST to override any other logic (e.g. if we are in a loop).
	if strings.Contains(strings.ToLower(prompt), "nothing to commit") || strings.Contains(strings.ToLower(prompt), "working tree clean") {
		return `It seems there are no changes to commit. The task is complete.

` + "```bash" + `
if command -v agent-bridge >/dev/null 2>&1; then
    agent-bridge signal COMPLETED true
else
    echo "agent-bridge not found, skipping signal"
fi
` + "```" + `
`, nil
	}

	// 2. Ticket Generation / Initializer Request (Prime Python Scenario)
	// Matches if asking for JSON format AND (ID:[PRIMES] OR ([GEN] and feature list))
	// This ensures Initializer gets JSON even if [GEN] tag is present (which usually triggers implementation)
	isPrimesScenario := strings.Contains(prompt, "ID:[PRIMES]") || (strings.Contains(prompt, "[GEN]") && strings.Contains(prompt, "Prime Number Script"))
	isJsonRequest := strings.Contains(prompt, "JSON format") || strings.Contains(prompt, "feature_list.json")

	if isPrimesScenario && isJsonRequest {
		// If asking for feature_list.json specifically, return FeatureList format
		if strings.Contains(prompt, "feature_list.json") {
			return "```json\n" + `{
  "features": [
    {
      "id": "[GEN] Create Prime Number Script",
      "title": "[GEN] Create Prime Number Script",
      "name": "[GEN] Create Prime Number Script",
      "description": "Create a python script named 'primes.py' that calculates primes < 10000 and outputs to 'primes.json'. ID:[PRIMES]",
      "category": "Core",
      "priority": "MVP",
      "status": "pending",
      "dependencies": {
        "depends_on_ids": []
      }
    }
  ]
}` + "\n```", nil
		}

		// Otherwise (Ticket Gen), return ticketNode format
		return `[
  {
    "title": "[GEN] Create Prime Number Script",
    "description": "Create a python script named 'primes.py' that calculates primes < 10000 and outputs to 'primes.json'. ID:[PRIMES]",
    "type": "Task",
    "children": []
  }
]`, nil
	}

	// 3. Implementation Request (Writing the file)
	// Matches prompt asking to implement "PRIMES" or "primes.py"
	// Also check for [GEN] tag which appears in E2E tests
	if strings.Contains(prompt, "PRIMES") || strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "[GEN]") {
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
git commit -m "Add primes script and output" || echo "Nothing to commit"

# Signal completion
agent-bridge feature set "[GEN] Create Prime Number Script" --status done --passes true
` + "```" + `
`, nil
	}

	// Default Mock Response
	// We include a no-op bash block to ensure the executor doesn't trip the "no commands" circuit breaker
	// We also signal completion to ensure unit tests with limited iterations finish successfully.
	// We strip backticks from the preview to avoid confusing the regex parser
	preview := strings.ReplaceAll(truncateString(prompt, 100), "`", "'")
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...\n\n```bash\n# no-op to prevent circuit breaker\necho 'mock agent alive'\nif command -v agent-bridge >/dev/null 2>&1; then agent-bridge signal COMPLETED true || echo 'Agent Bridge Failed'; fi\n```",
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
