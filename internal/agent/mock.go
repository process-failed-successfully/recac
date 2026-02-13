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
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristics for E2E scenarios
	lowerPrompt := strings.ToLower(prompt)

	// --- 1. Planning / Jira Generation Heuristics (High Priority) ---
	// If the prompt asks for ticket generation or mentions the TPM role, return a JSON array of tickets.
	// This MUST be checked before "prime python" implementation checks.
	if (strings.Contains(lowerPrompt, "technical program manager") ||
		strings.Contains(lowerPrompt, "generate ticket")) &&
		(strings.Contains(lowerPrompt, "prime") || strings.Contains(lowerPrompt, "python")) {

		// Extract repo URL from prompt if possible, or use a placeholder
		repoURL := "https://github.com/example/repo"
		re := regexp.MustCompile(`Repo: (https?://[^\s]+)`)
		matches := re.FindStringSubmatch(prompt)
		if len(matches) > 1 {
			repoURL = matches[1]
		}

		// Return JSON array of tickets as expected by 'jira generate-from-spec'
		// Note: The E2E test checks for specific ID tags like ID:[PRIMES] in the Story title.
		// We add ID:[PRIMES] to the Story title, NOT the Epic title, as the Orchestrator ignores Epics.
		return fmt.Sprintf(`[
  {
    "title": "Prime Number Generator Epic",
    "description": "Epic for prime number generator implementation",
    "type": "Epic",
    "status": "To Do",
    "priority": "High",
    "project_key": "MFLP",
    "labels": ["recac-agent"],
    "repo_url": "%s"
  },
  {
    "title": "ID:[PRIMES] Implement Prime Number Script",
    "description": "Create a python script to calculate primes up to 10000. The script should be named primes.py and print 'Found X primes'.",
    "type": "Story",
    "status": "To Do",
    "priority": "High",
    "project_key": "MFLP",
    "labels": ["recac-agent"],
    "repo_url": "%s",
    "dependencies": []
  }
]`, repoURL, repoURL), nil
	}

	// --- 2. Initializer Agent Heuristics (High Priority) ---
	// Initializer Agent is responsible for setting up the environment and dependencies.
	// It usually runs first if triggered.
	if (strings.Contains(lowerPrompt, "initializer agent") || strings.Contains(lowerPrompt, "initialize")) &&
		strings.Contains(lowerPrompt, "feature") {

		return `#!/bin/bash
set -e
set -o pipefail

# Initializer Agent Mock Response
echo '{"features": [{"id": "feature-1", "description": "Prime Number Generator", "status": "pending"}]}' | agent-bridge import
echo "Initialization complete."
`, nil
	}

	// --- 3. QA Agent Heuristics ---
	if strings.Contains(lowerPrompt, "qa agent") || strings.Contains(lowerPrompt, "quality assurance") {
		return `#!/bin/bash
set -e
echo "Running QA checks..."
# Simulate QA success
agent-bridge signal QA_PASSED true
echo "QA checks passed."
`, nil
	}

	// --- 4. Project Manager / Sign-off Heuristics ---
	if strings.Contains(lowerPrompt, "project manager") || strings.Contains(lowerPrompt, "sign off") {
		return `#!/bin/bash
set -e
echo "Reviewing project..."
# Simulate project sign-off
agent-bridge signal PROJECT_SIGNED_OFF true
echo "Project signed off."
`, nil
	}

	// --- 5. Implementation / Execution Heuristics (Lower Priority) ---
	// Prime Python Scenario (Implementation Phase)
	if strings.Contains(lowerPrompt, "prime") && (strings.Contains(lowerPrompt, "python") || strings.Contains(lowerPrompt, "script")) {
		// Check if we should create a file (Execution phase)
		return `I will create a python script to calculate primes.

` + "```bash" + `
cat << 'EOF' > primes.py
def is_prime(n):
    """Checks if a number is a prime number."""
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

count = 0
for i in range(10000):
    if is_prime(i):
        count += 1
print(f"Found {count} primes")

# Write result to json for verification
import json
with open("primes.json", "w") as f:
    json.dump({"primes": [p for p in range(10000) if is_prime(p)]}, f)
EOF

git add primes.py
git commit -m "Add primes script"
git push origin HEAD

# Signal completion to prevent infinite loops in stateless E2E
agent-bridge signal COMPLETED true
` + "```", nil
	}

	// Default fallback response
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
