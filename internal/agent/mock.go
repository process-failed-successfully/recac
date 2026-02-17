package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// Pre-compile regex for performance
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

	// Heuristics for E2E Tests (Smoke Test)

	// 1. Execution Phase (Check status pending)
	// We check this first to prevent "Technical Program Manager" role assumption in execution loop
	if pendingStatusRegex.MatchString(prompt) {
		return `
#!/bin/bash
# Complete the task
echo "Completing task..."
cat > feature_list.json <<EOF
{
  "project_name": "primes",
  "features": [
    {
      "id": "req-prime-generator",
      "description": "Implement prime calculation logic in primes.py",
      "status": "done",
      "verification_steps": ["python3 primes.py"]
    }
  ]
}
EOF
echo "Task completed"
`, nil
	}

	// 2. Planning Phase (Technical Program Manager)
	// Return a JSON list of tickets
	if strings.Contains(prompt, "Technical Program Manager") {
		return `
[
  {
    "id": "ID:[PRIMES]",
    "type": "task",
    "title": "Implement Prime Generator",
    "description": "Implement prime calculation logic in primes.py",
    "dependencies": []
  }
]
`, nil
	}

	// 3. Architect/Coding Phase (Lead Software Architect or Primes Task)
	if strings.Contains(prompt, "Lead Software Architect") || strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "Implement prime calculation logic") {
		return `
#!/bin/bash
# Create the python file
echo "Creating primes.py..."
cat > primes.py <<EOF
def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

if __name__ == "__main__":
    print(is_prime(7))
EOF

# Create the features file if not exists
if [ ! -f feature_list.json ]; then
cat > feature_list.json <<EOF
{
  "project_name": "primes",
  "features": [
    {
      "id": "req-prime-generator",
      "description": "Implement prime calculation logic in primes.py",
      "status": "pending",
      "verification_steps": ["python3 primes.py"]
    }
  ]
}
EOF
fi

# Git config (Crucial for CI)
git config user.email "bot@recac.io" || true
git config user.name "Recac Bot" || true

# Commit
git add primes.py feature_list.json || true
git commit -m "Implement prime generator" || echo "Nothing to commit"

echo "Success: Mock command executed"
`, nil
	}

	// 4. QA Agent
	if strings.Contains(prompt, "QA AGENT") {
		return `
#!/bin/bash
echo "Running QA..."
python3 primes.py
# Signal success
agent-bridge signal QA_PASSED true
`, nil
	}

	// 5. Manager Agent
	if strings.Contains(prompt, "Manager Agent") {
		return `
#!/bin/bash
echo "Signing off..."
agent-bridge signal PROJECT_SIGNED_OFF true
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
