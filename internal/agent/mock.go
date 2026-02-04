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

// Send implements the Agent interface
// It returns a mock response that acknowledges the prompt
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// 1. Handle TPM / Ticket Generation Prompt (JSON output)
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "generate-from-spec") {
		// Extract ID if present, e.g., ID:[PRIMES]
		reID := regexp.MustCompile(`ID:\[(.*?)\]`)
		matches := reID.FindStringSubmatch(prompt)
		id := "TASK-1"
		title := "Mock Task"
		if len(matches) > 1 {
			id = matches[1]
			title = fmt.Sprintf("ID:[%s] Mock Task", id)
		}

		// Return JSON for recac CLI
		// We extract extracted ID to ensure it maps correctly in the runner
		return fmt.Sprintf(`[
  {
    "title": "%s",
    "description": "Mock description for %s. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Task",
    "acceptance_criteria": ["Create primes.py"],
    "children": []
  }
]`, title, id), nil
	}

	// 2. Handle Initializer (feature_list.json) - if requested
	if strings.Contains(prompt, "Initialize") && strings.Contains(prompt, "feature_list.json") {
		// Use agent-bridge import to properly initialize features in the DB
		return "```bash\n" + `cat << 'EOF' | agent-bridge import
{
  "project_name": "mock-project",
  "features": [
    {"id": "req-primes-py-exists", "description": "primes.py exists", "status": "todo", "type": "file_exists", "target": "primes.py"}
  ]
}
EOF
` + "\n```", nil
	}

	// 3. Handle QA Agent
	// Matches: ## YOUR ROLE - QA AGENT
	if strings.Contains(prompt, "QA AGENT") {
		return "```bash\n# Mock QA Checks\nmake test || echo \"Tests failed\"\n\n# Signal Success\nagent-bridge signal QA_PASSED true\n```", nil
	}

	// 4. Handle Project Manager
	// Matches: ## YOUR ROLE - PROJECT MANAGER
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return "```bash\n# Mock Project Sign-Off\nagent-bridge signal PROJECT_SIGNED_OFF true\n```", nil
	}

	// 5. Handle Coding Agent (primes.py)
	if strings.Contains(prompt, "primes.py") {
		return "```bash\n" + `#!/bin/bash
set -e

# Configure git if needed
git config user.email "mock@agent.com" || true
git config user.name "Mock Agent" || true

# Create primes.py
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [i for i in range(10000) if is_prime(i)]

with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run it to generate output
python3 primes.py

# Add and commit
git add primes.py primes.json
git commit -m "Add primes.py and output" || echo "Nothing to commit"

# Signal completion if bridge is available
if command -v agent-bridge &> /dev/null; then
    # Try to update specific known IDs or discover them
    agent-bridge feature set req-primes-py-exists --status done --passes true || true
    agent-bridge feature set req-create-primes-py --status done --passes true || true

    # Fallback: Update all pending features
    if command -v jq &> /dev/null; then
        agent-bridge feature list --json | jq -r '.features[] | select(.status!="done") | .id' | xargs -r -I {} agent-bridge feature set {} --status done --passes true || true
    fi
fi
` + "\n```", nil
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
