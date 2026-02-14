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
// It returns a mock response based on prompt heuristics for E2E scenarios
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// 1. Project Manager / Planning Phase
	if strings.Contains(prompt, "role - Project Manager") || strings.Contains(prompt, "Technical Program Manager") {
		// Return JSON plan for the 'generate-from-spec' command
		// Note: The struct expects fields: title, description, type, blocked_by, acceptance_criteria
		return `{
  "tickets": [
    {
      "title": "Implement Prime Number Script",
      "description": "Create a python script 'primes.py' that calculates primes up to 100 and saves them to 'primes.json'.",
      "type": "Task",
      "id": "PRIMES-1",
      "acceptance_criteria": ["req-script-runs-without-errors"]
    }
  ]
}`, nil
	}

	// 2. QA Agent Phase
	if strings.Contains(prompt, "role - QA Agent") {
		return "QA Approved.\n```bash\nagent-bridge signal PROJECT_SIGNED_OFF true --privileged\n```", nil
	}

	// 3. Coding Agent Phase (Primes Scenario)
	// Detects if we are working on the primes task
	if strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "1229") {
		return `I will implement the prime number script as requested.

` + "```bash" + `
# Create the python script
cat <<EOF > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(100) if is_prime(x)]
with open("primes.json", "w") as f:
    json.dump(primes, f)
print(f"Generated {len(primes)} primes")
EOF

# Execute it to generate the output artifact
python3 primes.py

# Commit changes
git add primes.py primes.json
git commit -m "Add primes script implementation" || echo "nothing to commit"

# Mark requirement as done (to prevent premature sign-off revocation)
# We assume the requirement ID is req-script-runs-without-errors based on the planning phase
agent-bridge import --id req-script-runs-without-errors --description "Script runs without errors" --status completed
agent-bridge feature set req-script-runs-without-errors status completed

# Signal completion
# Note: We signal project sign-off here to ensure the smoke test completes even if QA step is skipped or merged
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
` + "```", nil
	}

	// Default Mock Response
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
