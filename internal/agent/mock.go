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

	// Apply heuristics for specific roles/tasks
	if response := m.applyHeuristics(prompt); response != "" {
		return response, nil
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

// applyHeuristics checks the prompt for specific keywords and returns a hardcoded response
// This is used for smoke tests and E2E scenarios where we need deterministic behavior without an LLM.
func (m *MockAgent) applyHeuristics(prompt string) string {
	p := strings.ToUpper(prompt)

	// 1. Initializer Agent
	if strings.Contains(p, "INITIALIZER AGENT") || strings.Contains(p, "YOU ARE THE INITIALIZER") {
		return `#!/bin/bash
echo "Initializing project environment..."
git config --global user.email "agent@recac.ai"
git config --global user.name "Recac Agent"
git init
echo "# Project" > README.md
git add README.md
git commit -m "chore: initial commit" || echo "Nothing to commit"
# Push if remote exists
git push origin HEAD || echo "No remote configured or push failed"
echo "Initialization complete."
`
	}

	// 2. TPM (Technical Program Manager) - Generates Ticket Plan
	if strings.Contains(p, "TPM") || strings.Contains(p, "TECHNICAL PROGRAM MANAGER") {
		// Return a JSON plan for the smoke test
		return `[
  {
    "title": "ID:[INIT] Setup Project Infrastructure",
    "description": "Initialize the repository and basic configuration.",
    "type": "Epic",
    "children": []
  },
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Create a Python script named primes.py that calculates prime numbers up to 10000. It must output a JSON object {\"primes\": [...]} to stdout when executed.",
    "type": "Story",
    "children": []
  }
]`
	}

	// 3. QA Agent
	if strings.Contains(p, "QA AGENT") || strings.Contains(p, "QUALITY ASSURANCE") {
		return "agent-bridge signal QA_PASSED true"
	}

	// 4. Project Manager
	if strings.Contains(p, "PROJECT MANAGER") || strings.Contains(p, "MANAGER") {
		return "agent-bridge signal PROJECT_SIGNED_OFF true"
	}

	// 5. Coding Agent - Handle specific tasks
	if strings.Contains(p, "CODING AGENT") || strings.Contains(p, "DEVELOPER") {
		// Primes Scenario
		if strings.Contains(p, "PRIMES") || strings.Contains(p, "PRIME NUMBER") {
			return `cat <<EOF > primes.py
import json

def generate_primes(n):
    primes = []
    for num in range(2, n):
        is_prime = True
        for i in range(2, int(num ** 0.5) + 1):
            if num % i == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(num)
    return primes

if __name__ == "__main__":
    # Generate primes up to 10000 as requested
    result = {"primes": generate_primes(10000)}
    print(json.dumps(result))
EOF

# Verify it works
python3 primes.py > /dev/null

# Commit and Push
git config --global user.email "agent@recac.ai"
git config --global user.name "Recac Agent"
git add primes.py
git commit -m "feat: implement prime generator" || echo "Nothing to commit"
git push origin HEAD

# Signal Completion
agent-bridge signal COMPLETED true
`
		}

		// Loop Breaker / Generic Task Completion
		if strings.Contains(p, "NOTHING TO COMMIT") || strings.Contains(p, "CLEAN") {
			return "agent-bridge signal QA_PASSED true\nagent-bridge signal PROJECT_SIGNED_OFF true"
		}

		// Default Coding Action: Try to commit any changes or just pass
		return `echo "Coding Agent: Work in progress..."
git add .
git commit -m "wip: agent progress" || echo "Nothing to commit"
git push origin HEAD || echo "Push skipped"
agent-bridge signal COMPLETED true
`
	}

	return ""
}
