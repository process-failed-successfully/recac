package agent

import (
	"context"
	"fmt"
	"regexp"
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

var taskIDRegex = regexp.MustCompile(`Task ID: (\S+)`)

// Send implements the Agent interface
// It returns a mock response that acknowledges the prompt
// This allows the session to run without requiring real API keys
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// 1. Check for TPM Agent (Planning Phase)
	if strings.Contains(prompt, "Technical Program Manager") {
		// Return hardcoded JSON ticket list for "Technical Program Manager"
		return `[
  {
    "title": "ID:[PRIMES] Prime Number Generator",
    "description": "Implement a script to generate prime numbers. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Epic",
    "children": [
      {
        "title": "Implement Prime Script",
        "description": "Create a python script to generate primes. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
        "type": "Story",
		"acceptance_criteria": ["Script exists", "Generates primes"]
      }
    ]
  }
]`, nil
	}

	// 2. Check for Coding Agent (Execution Phase)
	if strings.Contains(prompt, "YOUR ROLE - CODING AGENT") {
		// Heuristic: If prompt contains "pending", we are likely in execution loop
		// But for the specific PRIMES task, we want to just do it.
		// The prompt contains "Task ID: {task_id}".

		matches := taskIDRegex.FindStringSubmatch(prompt)
		taskID := "UNKNOWN"
		if len(matches) > 1 {
			taskID = matches[1]
		}

		// For PRIMES, return a bash script to create the file and mark done
		if strings.Contains(taskID, "PRIMES") || strings.Contains(prompt, "Prime Number") {
			return `
Here is the implementation for the prime number generator.

` + "```bash" + `
# Configure git
git config user.email "agent@recac.io"
git config user.name "Recac Agent"

# Create the python script
cat << 'EOF' > primes.py
def generate_primes(n):
    primes = []
    for i in range(2, n + 1):
        for j in range(2, int(i ** 0.5) + 1):
            if i % j == 0:
                break
        else:
            primes.append(i)
    return primes

if __name__ == "__main__":
    import json
    print(json.dumps(generate_primes(100)))
EOF

# Create output json
python3 primes.py > primes.json

# Commit
git add primes.py primes.json || echo "Nothing to add"
git commit -m "Implement primes generator" || echo "Nothing to commit"

# Mark as done
agent-bridge feature set ` + taskID + ` --status done --passes true
echo "Success: Mock command executed"
` + "```" + `
`, nil
		}

		// Fallback for generic tasks - just mark as done to avoid infinite loop
		return `
` + "```bash" + `
echo "Mock agent execution for task ` + taskID + `"
agent-bridge feature set ` + taskID + ` --status done --passes true
` + "```" + `
`, nil
	}

	// Default fallback
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
