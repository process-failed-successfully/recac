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

	// 1. TPM Agent Heuristic
	// Detects TPM role by checking for "Technical Program Manager" and "Application Specification"
	if strings.Contains(prompt, "Technical Program Manager") && strings.Contains(prompt, "Application Specification") {
		if strings.Contains(prompt, "[PRIMES]") {
			return `[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Create a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Task",
    "children": []
  }
]`, nil
		}
	}

	// 2. Initializer Heuristic
	// If the prompt is for Initializer (often contains "Initial Feature List"), imports the features.
	if strings.Contains(prompt, "INITIALIZER") || (strings.Contains(prompt, "Feature List") && strings.Contains(prompt, "agent-bridge import")) {
		// Extract JSON list from prompt
		jsonContent := extractJSON(prompt)
		if jsonContent != "[]" {
			return `I will initialize the feature list.

` + "```bash" + `
cat << 'EOF' > feature_list.json
` + jsonContent + `
EOF
agent-bridge import < feature_list.json
` + "```" + `
`, nil
		}
	}

	// 3. Coding Agent Heuristic
	// Detects Coding Agent role and [PRIMES] task
	if (strings.Contains(prompt, "YOUR ROLE - CODING AGENT") || strings.Contains(prompt, "Role: Coding Agent")) &&
	   (strings.Contains(prompt, "PRIMES") || strings.Contains(prompt, "Prime Number Script")) {

		// If git status shows clean, we might be done, but let's just force the implementation for robustness
		// Loop Breaker: if "nothing to commit" is in the prompt (usually in history), signal done
		if strings.Contains(prompt, "nothing to commit") || strings.Contains(prompt, "working tree clean") {
			return `It seems the work is committed. I will mark the feature as done.

` + "```bash" + `
agent-bridge feature set PRIMES --status done --passes true
` + "```" + `
`, nil
		}

		// Otherwise, implement the script
		return `I will implement the prime number script as requested.

` + "```bash" + `
# Create the python script
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(10000) if is_prime(x)]
with open('primes.json', 'w') as f:
    json.dump({'primes': primes}, f)
EOF

# Run it to generate the JSON
python3 primes.py

# Commit the files
git add primes.py primes.json
git commit -m "Implement [PRIMES] prime number script"

# Update status
agent-bridge feature set PRIMES --status done --passes true
` + "```" + `
`, nil
	}

	// 4. Manager / QA Heuristic
	// Detects Manager or QA role and signs off
	if strings.Contains(prompt, "PROJECT MANAGER") || strings.Contains(prompt, "QA AGENT") {
		return `I have reviewed the work and it looks correct.

` + "```bash" + `
agent-bridge signal --privileged PROJECT_SIGNED_OFF true
agent-bridge signal QA_PASSED true
` + "```" + `
`, nil
	}

	// Default Fallback
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

// extractJSON attempts to extract a JSON array from the string
func extractJSON(s string) string {
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start != -1 && end != -1 && end > start {
		return s[start : end+1]
	}
	return "[]"
}
