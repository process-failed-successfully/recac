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

	// Helper for case-insensitive matching
	promptLower := strings.ToLower(prompt)

	// Heuristic for Initializer (feature_list.json)
	// Must check this BEFORE planning, because the Initializer prompt often includes the AppSpec.
	// We strictly check for "initializer agent" to match the role definition in the prompt template.
	// We REMOVE the brittle "!plan" exclusion because "explanation" contains "plan" which caused false negatives.
	if strings.Contains(promptLower, "feature_list.json") &&
		(strings.Contains(promptLower, "create") || strings.Contains(promptLower, "initialize")) &&
		strings.Contains(promptLower, "initializer agent") {
		return `
I will create the feature list.

` + "```bash" + `
set -x # Enable debug logging

cat << 'EOF' > feature_list.json
{
    "project_name": "Prime Number Script",
    "features": [
        {
            "id": "1",
            "category": "core",
            "priority": "MVP",
            "description": "Calculate primes under 10000",
            "status": "todo",
            "passes": false,
            "steps": [],
            "dependencies": {
                "depends_on_ids": [],
                "exclusive_write_paths": [],
                "read_only_paths": []
            }
        }
    ]
}
EOF

# Import it with output capture for debugging
# We don't fail if import fails because in mock/smoke-test environments (e.g. CI without DB),
# we still want the file to exist so the session can continue using the file-based source.
if command -v agent-bridge >/dev/null 2>&1; then
    agent-bridge import --file feature_list.json > import.log 2>&1 || echo "Warning: agent-bridge import failed (ignoring in mock mode)"
    cat import.log
else
    echo "Warning: agent-bridge not found, skipping import."
fi
` + "```" + `
`, nil
	}

	// Heuristic for "prime-python" scenario planning phase.
	// The planner prompt includes the AppSpec.
	// We check for the specific ID used in the spec.
	// We EXCLUDE "initializer agent" to ensure we don't accidentally hijack the Initializer phase
	// if it happens to contain the spec ID (which it does).
	if strings.Contains(prompt, "ID:[PRIMES]") &&
		(strings.Contains(promptLower, "appspec") || strings.Contains(promptLower, "specification")) &&
		!strings.Contains(promptLower, "initializer agent") {
		// Return a JSON ARRAY of tickets, as expected by cmd/recac/jira.go
		return `[
    {
      "title": "ID:[PRIMES] Create Prime Number Script",
      "description": "Create a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to 'primes.json'.",
      "type": "Task"
    }
  ]`, nil
	}

	// Heuristic for "prime-python" scenario execution phase.
	// We want to match the task execution prompt but NOT the planning prompt.
	// The planning prompt also contains "primes.py" and "Create", so we must exclude it.
	isPlanning := strings.Contains(promptLower, "appspec") || strings.Contains(promptLower, "specification")

	// Check for implementation triggers:
	// 1. Task ID: [PRIMES] (often in prompt as "Task: [PRIMES]" or similar)
	// 2. File + Action: "primes.py" AND "create" (case insensitive)
	isImplementation := strings.Contains(prompt, "[PRIMES]") || (strings.Contains(promptLower, "primes.py") && strings.Contains(promptLower, "create"))

	if !isPlanning && isImplementation {
		return `
I will implement the prime number script.

` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [p for p in range(10000) if is_prime(p)]

with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py

# Add to git
git add primes.py primes.json
git commit -m "Add primes.py and primes.json"
` + "```" + `
`, nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
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
