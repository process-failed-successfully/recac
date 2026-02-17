package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var pendingStatusRegex = regexp.MustCompile(`"status":\s*"pending"`)

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

	// Check for "status": "pending" to complete tasks (Execution Phase)
	// Using regex to allow for whitespace variations
	if pendingStatusRegex.MatchString(prompt) {
		// Return script to mark task as done
		script := `#!/bin/bash
# Mock Agent: Marking task as completed
cat <<EOF > feature_list.json
[
  {
    "id": "primes",
    "description": "Implement prime calculation logic in primes.py",
    "status": "done"
  }
]
EOF
echo "Updated feature_list.json status to done"
`
		return fmt.Sprintf("I see there is a pending task. I have completed the work and will update the status.\n\n```bash\n%s\n```", script), nil
	}

	// Check for Planning Phase (Technical Program Manager)
	if strings.Contains(prompt, "Technical Program Manager") {
		json := `[
  {
    "title": "ID:[PRIMES] Prime Number Generator",
    "description": "Implement prime calculation logic in primes.py",
    "type": "Epic",
    "children": []
  }
]`
		return fmt.Sprintf("Here is the plan:\n```json\n%s\n```", json), nil
	}

	// Check for Prime Calculation Task (Coding Phase)
	if strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "Implement prime calculation logic") {
		// Return bash script to create primes.py and generate primes.json
		script := `#!/bin/bash
# Mock Agent: Generating primes.py

# Configure git if needed (for CI environments)
git config user.email "mock@agent.com" || true
git config user.name "Mock Agent" || true

cat <<EOF > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [n for n in range(10000) if is_prime(n)]

with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)

print(f"Generated {len(primes)} primes to primes.json")
EOF

# Verify it works
python3 primes.py

# Commit changes
git add primes.py primes.json || echo "git add failed"
git commit -m "Implement primes.py and generate primes.json" || echo "git commit failed (nothing to commit?)"

echo "Success: Mock command executed"
`
		return fmt.Sprintf("I will implement the prime calculation logic.\n\n```bash\n%s\n```", script), nil
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
