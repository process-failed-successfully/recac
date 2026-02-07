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

	// Heuristic: Check for "TPM" or "Technical Program Manager" role to generate a ticket plan
	if strings.Contains(prompt, "TPM") || strings.Contains(prompt, "Technical Program Manager") {
		// Return a valid JSON response for ticket generation (Array of tickets)
		return `[
    {
      "title": "Implement Core Feature",
      "description": "Implement the core functionality as requested.",
      "type": "Epic",
      "children": [
        {
          "title": "Setup Project Structure",
          "description": "Initialize the project structure.",
          "type": "Story"
        },
        {
          "title": "Implement Logic",
          "description": "Write the business logic.",
          "type": "Story"
        }
      ]
    }
  ]`, nil
	}

	// Heuristic: Check if this is the Initializer agent
	// Explicitly exclude prompts containing "CODING AGENT" because the coding agent prompt
	// naturally contains "feature_list.json" in its instructions, which would trigger a false positive.
	isInitializer := strings.Contains(prompt, "Initializer") || strings.Contains(prompt, "feature_list.json")
	if isInitializer && !strings.Contains(prompt, "CODING AGENT") {
		// Special handling for the Prime Python scenario
		// We check for [PRIMES] (Planner mode) or "Prime Number" (Jira mode where ID is replaced)
		if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "Prime Number") || strings.Contains(prompt, "primes.py") {
			return `Mock Initializer: Creating feature list for [PRIMES].
` + "```bash" + `
echo '[{"id": "PRIMES", "description": "Create a python script named primes.py that calculates all prime numbers less than 10,000 and outputs them to primes.json.", "status": "todo", "file_paths": []}]' > feature_list.json && agent-bridge import feature_list.json || echo 'Bridge skipped'
` + "```", nil
		}

		// Create the file AND import it to DB to satisfy loadFeatures
		// We must provide a non-empty list so agent-bridge import succeeds
		return "Mock Initializer: Creating feature list.\n```bash\necho '[{\"id\": \"mock-feature\", \"description\": \"A mock feature for testing\", \"status\": \"todo\", \"file_paths\": []}]' > feature_list.json && agent-bridge import feature_list.json || echo 'Bridge skipped'\n```", nil
	}

	// Heuristic: Check for Prime Number script task (used in smoke tests/CI)
	if strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "Prime Number") || strings.Contains(prompt, "[PRIMES]") {
		// Return a bash script that implements the prime number calculator
		// This must satisfy the verification logic in pkg/e2e/scenarios/prime_python.go
		return `Mock Agent: Implementing prime number script.
` + "```bash" + `
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [x for x in range(10000) if is_prime(x)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run the script to generate the output
python3 primes.py

# Configure git if needed (for CI environments)
git config user.email "mock-agent@example.com"
git config user.name "Mock Agent"

# Commit the changes
git add primes.py primes.json
git commit -m "Implement prime number calculator"
` + "```", nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...\n\n```bash\necho 'Mock Agent: Processing request...'\n```",
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
