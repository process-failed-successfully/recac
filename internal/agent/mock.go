package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var (
	// Regex to detect execution phase prompt for a pending feature
	pendingStatusRegex = regexp.MustCompile(`"status":\s*"pending"`)
	// Regex to extract Task ID from prompt
	taskIDRegex = regexp.MustCompile(`Task ID: (\S+)`)
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

	// Extract Task ID
	var taskID string
	if matches := taskIDRegex.FindStringSubmatch(prompt); len(matches) > 1 {
		taskID = matches[1]
	}

	// 1. Check for Prime Number Task (Smoke Test)
	if strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "Implement prime calculation logic") {
		return m.generatePrimesScript(taskID), nil
	}

	// 2. Check for Planning Phase (TPM)
	if strings.Contains(prompt, "Technical Program Manager") {
		return m.generatePlan(), nil
	}

	// 3. Check for Execution Phase (Pending Task)
	if pendingStatusRegex.MatchString(prompt) && taskID != "" {
		return m.generateExecutionScript(taskID), nil
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

func (m *MockAgent) generatePrimesScript(taskID string) string {
	script := `
I will implement the prime number calculation script as requested.

` + "```bash" + `
#!/bin/bash
set -e

# Configure git
git config --global user.email "bot@recac.com"
git config --global user.name "Recac Bot"

# Create primes.py
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [i for i in range(10000) if is_prime(i)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run the script to generate primes.json
python3 primes.py

# Add and commit
git add primes.py primes.json
git commit -m "Add prime number calculation script and results" || echo "Nothing to commit"

echo "Success: Mock command executed"
` + "```" + `
`
	if taskID != "" {
		script += fmt.Sprintf(`
`+"```bash"+`
# Mark task as done
if command -v agent-bridge &> /dev/null; then
    agent-bridge feature set %s --status done --passes true
else
    echo "agent-bridge not found, cannot update status"
fi
`+"```"+`
`, taskID)
	}
	return script
}

func (m *MockAgent) generatePlan() string {
	return `
I have reviewed the plan/code. Everything looks good.

` + "```bash" + `
# Signal project sign-off
if command -v agent-bridge &> /dev/null; then
    agent-bridge signal set PROJECT_SIGNED_OFF true
else
    echo "agent-bridge not found"
fi
` + "```" + `
`
}

func (m *MockAgent) generateExecutionScript(taskID string) string {
	return fmt.Sprintf(`
I will mark the task %s as done.

`+"```bash"+`
#!/bin/bash
# Mock execution step
echo "Executing task %s..."
if command -v agent-bridge &> /dev/null; then
    agent-bridge feature set %s --status done --passes true
else
    echo "agent-bridge not found"
fi
echo "Success: Mock command executed"
`+"```"+`
`, taskID, taskID, taskID)
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
