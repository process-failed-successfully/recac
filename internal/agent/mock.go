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
	forcedError    error
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

// SetError forces a specific error from the agent
func (m *MockAgent) SetError(err error) {
	m.forcedError = err
}

// Send implements the Agent interface
// It returns a mock response that acknowledges the prompt
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedError != nil {
		return "", m.forcedError
	}
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Smart Mocking for Smoke Tests
	// If the prompt looks like the Prime Python spec, we need to handle two cases:
	// 1. Planning phase (asking for tickets/plan) -> Return JSON plan
	// 2. Implementation phase (asking for code) -> Return Bash script

	if strings.Contains(prompt, "ID:[PRIMES]") {
		// Check for planner keywords
		// The prompt template for planner usually includes the spec but asks for a plan/tickets.
		// "generate a detailed JSON feature list" or similar.
		// Since we don't know the exact prompt template, we look for key differentiators.
		// The coding agent prompt usually starts with "You are an autonomous coding agent" or similar.
		// The planner prompt usually mentions "plan" or "tickets" prominently.
		//
		// Simpler heuristic: If the prompt contains "JSON" and "tickets" or "feature list", it's likely planning.
		// Or if it DOES NOT contain "bash block" or "implementation".

		// Let's assume the Planner phase comes first and asks for a JSON plan.
		// The log showed the planner returned the bash script, causing invalid JSON error.
		// This means my previous condition caught the planner prompt.

		// Let's try to detect the "Planner" context.
		// e2e/runner/main.go calls GenerateScenario which calls recac plan.
		// If we check for "plan" or "feature list", we return the JSON.

		if strings.Contains(strings.ToLower(prompt), "plan") || strings.Contains(strings.ToLower(prompt), "feature list") {
			return `[
  {
    "title": "ID:[PRIMES] Create Prime Number Script",
    "description": "Create a python script named 'primes.py'. It MUST be python.\nIt must calculate all prime numbers less than 10,000 and output to a file named 'primes.json'.\nIMPORTANT: You MUST use a bash block to create the file (e.g., cat << 'EOF' > primes.py). Do not output raw python code.\nCommit 'primes.py' and 'primes.json' IMMEDIATELY. Use 'git add -f primes.json' to ensure it is tracked.\nThe JSON format must have a single key 'primes' containing the list of integers.\nExample: {\"primes\": [2, 3, 5, ...]}.\nIMPORTANT: Ensure the FINAL primes.json committed to the repository contains ALL primes less than 10,000 (Exactly 1229 primes).\nDo not truncate it for testing or reporting - the verification script expects the full list.\nKeep the code absolutely minimal. Finish as quickly as possible.\n\nCRITICAL: You MUST name the script 'primes.py'. Do not use 'feature_implementation.py' or any other generic name.\nCRITICAL: Do NOT run 'pytest' or any test framework. Do NOT try to create test files. Just run the script and verify 'primes.json' exists.",
    "type": "Task",
    "children": []
  }
]`, nil
		}

		// Otherwise, return the implementation
		return `I will create the prime number script as requested.

` + "```bash" + `
# Create the python script
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(10000) if is_prime(x)]
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run the script to generate the json
python3 primes.py

# Commit the files
git add primes.py primes.json
git commit -m "feat: add primes script"
` + "```" + `
`, nil
	}

	if strings.Contains(prompt, "Create a python script named 'primes.py'") {
		return `I will implement the prime number script as requested.

` + "```bash" + `
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
    json.dump({'primes': primes}, f)
EOF

python3 primes.py
git add primes.py primes.json
git commit -m "Add primes implementation"
git push

# Mark project as signed off to trigger early exit in smoke tests
recac signal set PROJECT_SIGNED_OFF true
` + "```" + `
`, nil
	}

	if strings.Contains(prompt, "Create a guided tour of this codebase") {
		return `[
  {
    "title": "Project Overview",
    "filepath": "README.md",
    "description": "# Project Overview\n\nWelcome to **recac**! This is a distributed autonomous coding framework.\n\nKey features:\n- **Orchestrator**: Manages tasks and agents.\n- **Agent**: The core autonomous coding logic.\n- **CLI**: The command-line interface you are using now."
  },
  {
    "title": "Entry Point",
    "filepath": "cmd/orchestrator/main.go",
    "description": "This is the main entry point for the orchestrator."
  },
  {
    "title": "Core Logic",
    "filepath": "internal/runner/runner.go",
    "description": "The ` + "`runner`" + ` package contains the core loop for the autonomous agent. It manages the session, state, and execution of actions."
  }
]`, nil
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
