package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
)

// MockAgent is a smart mock agent for testing
// It returns context-aware responses based on the prompt content
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

	// Log the prompt for debugging CI failures
	log.Printf("[MockAgent] Received Prompt (len=%d): %s...", len(prompt), truncateString(prompt, 200))

	// 1. TPM Role (Planning)
	if strings.Contains(prompt, "You are an expert Technical Program Manager (TPM)") || strings.Contains(prompt, "## YOUR ROLE - PROJECT MANAGER") {
		// Return a JSON plan for the PRIMES task
		return `[
  {
    "id": "PRIMES",
    "title": "ID:[PRIMES] Implement Primes",
    "description": "Implement prime number calculation script",
    "type": "Task",
    "status": "todo",
    "dependencies": []
  }
]`, nil
	}

	// 2. Initializer Agent
	if strings.Contains(prompt, "## YOUR ROLE - INITIALIZER AGENT") {
		return `#!/bin/bash
cat << 'EOF' | agent-bridge import
[
  {
    "id": "PRIMES",
    "title": "ID:[PRIMES] Implement Primes",
    "description": "Implement prime number calculation script",
    "type": "Task",
    "status": "todo"
  }
]
EOF
`, nil
	}

	// 3. Coding Agent (Implementation)
	// Detects the specific task via [PRIMES] tag or keywords.
	// Relaxed matching: check for "primes.py" or "Prime" case-insensitive to be robust against prompt formatting changes.
	upperPrompt := strings.ToUpper(prompt)
	if strings.Contains(prompt, "## YOUR ROLE - CODING AGENT") || strings.Contains(upperPrompt, "[PRIMES]") || strings.Contains(prompt, "primes.py") || strings.Contains(upperPrompt, "PRIME") {
		// Python script to calculate primes < 10000
		// Note: We use %% for modulo to escape it in potential Sprintf usage, though here it's a raw string return.
		// However, to be safe and clear, we just return the string directly.
		pythonScript := `
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [x for x in range(10000) if is_prime(x)]
with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)
print(f"Generated {len(primes)} primes")
`
		// Escape single quotes for bash heredoc
		pythonScript = strings.ReplaceAll(pythonScript, "'", "'\\''")

		return fmt.Sprintf(`I will implement the prime number script as requested.

`+"```bash"+`
# Create the python script
cat << 'EOF' > primes.py
%s
EOF

# Run the script to generate the JSON
python3 primes.py

# Verify output exists
ls -l primes.json

# Commit the files
git add primes.py primes.json
git commit -m "Implement primes.py and generate primes.json"

# Signal completion
agent-bridge feature update PRIMES --status completed
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
`+"```"+`
`, pythonScript), nil
	}

	// 4. QA / Manager / Reviewer
	// Relaxed matching: Check for "QA" or "Review" case-insensitive.
	if strings.Contains(prompt, "## YOUR ROLE - QA AGENT") || strings.Contains(prompt, "## YOUR ROLE - PROJECT MANAGER") || strings.Contains(upperPrompt, "QA") || strings.Contains(upperPrompt, "REVIEW") {
		return `
Looking good! The implementation meets the requirements.

`+"```bash"+`
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
`+"```"+`
`, nil
	}

	// Default echo response for unknown prompts
	return fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100)), nil
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
