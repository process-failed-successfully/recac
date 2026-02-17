package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var (
	// pendingStatusRegex checks for "status": "pending" in the prompt
	pendingStatusRegex = regexp.MustCompile(`"status":\s*"pending"`)
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

	// Heuristic 1: Execution Phase (pending status)
	// Check this first because the prompt might contain other keywords
	if pendingStatusRegex.MatchString(prompt) {
		return `#!/bin/bash
# Mock execution script
echo "Updating feature status..."
# Use sed to update status from pending to done in feature_list.json
# Robustly handle if file doesn't exist (though it should)
if [ -f feature_list.json ]; then
  sed -i 's/"status": "pending"/"status": "done"/' feature_list.json
else
  echo "feature_list.json not found"
fi
echo "Success: Mock command executed"
`, nil
	}

	// Heuristic 2: Planning Phase (Technical Program Manager)
	if strings.Contains(prompt, "Technical Program Manager") {
		return `[
  {
    "id": "1",
    "title": "Implement Prime Number Generator",
    "description": "Create a python script that generates prime numbers.",
    "status": "pending",
    "assigned_to": "architect"
  }
]`, nil
	}

	// Heuristic 3: Architecture/Primes Task
	if strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "Lead Software Architect") || strings.Contains(prompt, "QA AGENT") {
		return `#!/bin/bash
# Create primes.py
cat <<EOF > primes.py
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

import json
primes = [x for x in range(100) if is_prime(x)]
print(json.dumps(primes))
with open('primes.json', 'w') as f:
    json.dump(primes, f)
EOF

# Create primes.json (empty initially or pre-filled)
echo "[]" > primes.json

# Git configuration (robustness for CI)
git config user.email "mock@recac.io"
git config user.name "Mock Agent"

# Git commit
git add primes.py primes.json
git commit -m "Add primes generator" || echo "Nothing to commit"

echo "Success: Mock command executed"
`, nil
	}

	// Default response
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
