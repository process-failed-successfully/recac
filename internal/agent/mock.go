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

	// 1. TPM / Ticket Generation (Priority)
	// Check for TPM specific keyword from tpm_agent.md template
	if strings.Contains(prompt, "Technical Program Manager (TPM)") {
		return m.handleTPM(), nil
	}

	lowerPrompt := strings.ToLower(prompt)

	// 2. Initializer / Feature List Phase
	// Check for "INITIALIZER AGENT" from initializer.md template
	// This prevents false positives where "feature_list.json" just appears in the context
	if strings.Contains(prompt, "INITIALIZER AGENT") {
		return m.handleInitializer(), nil
	}

	// Legacy fallback: if using an old prompt or just checking for "Initialize" command specifically
	// But be careful not to trigger on file presence.
	// Only trigger if it seems to be an instruction
	if strings.Contains(lowerPrompt, "create feature_list.json") || strings.Contains(lowerPrompt, "create init.sh") {
		return m.handleInitializer(), nil
	}

	// 3. Prime Python Scenario
	// Check for [PRIMES] tag or primes.py filename in an instruction context
	if strings.Contains(prompt, "[PRIMES]") || strings.Contains(lowerPrompt, "primes.py") {
		return m.handlePrimes(), nil
	}

	// 4. Manager/QA (Just pass)
	if strings.Contains(lowerPrompt, "qa agent") || strings.Contains(lowerPrompt, "project manager") {
		return "LGTM. QA Passed.", nil
	}

	// Return a mock response that shows the agent received the prompt
	// This allows the session to run without requiring real API keys
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func (m *MockAgent) handleTPM() string {
	return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Create a script to generate prime numbers. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Epic",
    "children": [
      {
        "title": "Create primes.py",
        "description": "Implement the Sieve of Eratosthenes or similar. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
        "type": "Story",
        "acceptance_criteria": [
          "Script runs without errors",
          "Generates primes < 10000"
        ],
        "blocked_by": []
      }
    ]
  }
]`
}

func (m *MockAgent) handleInitializer() string {
	return `I will initialize the feature list.
` + "```bash" + `
cat << 'EOF' > feature_list.json
[
  {
    "id": "req-primes",
    "description": "Calculate primes",
    "status": "todo",
    "priority": "MVP",
    "category": "functional",
    "steps": ["Run python3 primes.py", "Check primes.json"],
    "dependencies": {"exclusive_write_paths": ["primes.py"]}
  }
]
EOF

# Ensure agent-bridge exists (optional check)
if command -v agent-bridge &> /dev/null; then
    cat feature_list.json | agent-bridge import
    # Also update locally if needed, but import is key per new prompt
fi
` + "```"
}

func (m *MockAgent) handlePrimes() string {
	// The python script must generate exactly 1229 primes < 10000
	pythonScript := `
import json

def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [p for p in range(10000) if is_prime(p)]
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
`
	return `I will implement the primes script.
` + "```bash" + `
# Configure Git
git config user.email "bot@recac.com"
git config user.name "Recac Bot"

# Create Script
cat << 'EOF' > primes.py
` + pythonScript + `
EOF

# Run Script
python3 primes.py

# Commit
git add primes.py primes.json
git commit -m "Add primes.py and primes.json" || echo "Nothing to commit"

# Update status (if agent-bridge exists)
if command -v agent-bridge &> /dev/null; then
    agent-bridge update "req-primes" --status done
fi
` + "```"
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
