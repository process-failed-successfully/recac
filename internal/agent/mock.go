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

	// Heuristic: If the prompt asks for a Jira ticket plan (TPM persona), return a JSON list of tickets.
	// This fixes CI failures where 'recac jira generate-from-spec' expects strictly valid JSON.
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") {
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Script",
    "description": "Implement the core functionality based on the specification.",
    "type": "Task",
    "priority": "High"
  }
]`, nil
	}

	// Heuristic: If the prompt is for the Initializer Agent, return a feature plan.
	if strings.Contains(strings.ToLower(prompt), "initializer agent") {
		return `I will generate the feature list.

` + "```bash" + `
cat << 'EOF' | agent-bridge import
{
  "project_name": "Prime Generator",
  "features": [
    {
      "id": "req-script-runs-without-errors",
      "category": "functional",
      "priority": "MVP",
      "description": "Script runs without errors",
      "status": "pending",
      "passes": false,
      "steps": ["Run python script"],
      "dependencies": {
        "exclusive_write_paths": ["primes.py"],
        "read_only_paths": []
      }
    }
  ]
}
EOF

cat << 'EOF' > init.sh
#!/bin/bash
echo "Initializing..."
EOF
chmod +x init.sh
./init.sh
git init
git add .
git commit -m "Initial commit" || echo "Nothing to commit"
` + "```" + `
`, nil
	}

	// Heuristic: QA Agent - Always pass
	if strings.Contains(prompt, "QA AGENT") {
		return `I have run the tests and they passed.

` + "```bash" + `
echo "Running tests..."
# Simulate successful test run
agent-bridge signal QA_PASSED true
` + "```" + `
`, nil
	}

	// Heuristic: Manager - Always approve if QA passed (assumed)
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return `QA has passed and features are complete. I approve.

` + "```bash" + `
agent-bridge signal PROJECT_SIGNED_OFF true
` + "```" + `
`, nil
	}

	// Heuristic: If the prompt looks like a coding task (Agent persona), return a script that "does work"
	// to avoid "No-Op" circuit breaker failures in E2E tests.
	// EXCLUSION: Do not trigger if this is the Initializer Agent (handled above).
	if (strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "primes.py")) && !strings.Contains(strings.ToLower(prompt), "initializer agent") {
		return `I will implement the prime number generator.

` + "```bash" + `
# Create the python script
cat <<EOF > primes.py
import sys
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [i for i in range(1, 101) if is_prime(i)]
with open('primes.json', 'w') as f:
    json.dump(primes, f)
print(f"Generated {len(primes)} primes")
EOF

# Run the script
python3 primes.py

# Commit results
git add primes.py primes.json
git commit -m "feat: implement prime generator" || echo "Nothing to commit"

# Signal completion
echo '{"features": [{"id": "req-script-runs-without-errors", "type": "requirement", "description": "Script runs without errors", "status": "pending", "passes": false}]}' | agent-bridge import
agent-bridge feature set --id "req-script-runs-without-errors" --status "done" --passes true
agent-bridge signal --privileged PROJECT_SIGNED_OFF
` + "```" + `
`, nil
	}

	// Default: Return a mock text response
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
