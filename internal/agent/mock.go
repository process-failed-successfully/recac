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

	upperPrompt := strings.ToUpper(prompt)
	// Log prompt analysis for debugging
	log.Printf("[MockAgent] Prompt Analysis (len=%d):", len(prompt))
	log.Printf("  - Contains 'TPM': %v", strings.Contains(prompt, "Technical Program Manager"))
	log.Printf("  - Contains 'INITIALIZER': %v", strings.Contains(prompt, "INITIALIZER AGENT"))
	log.Printf("  - Contains 'CODING': %v", strings.Contains(prompt, "CODING AGENT"))
	log.Printf("  - Contains 'PROJECT MANAGER': %v", strings.Contains(prompt, "PROJECT MANAGER"))
	log.Printf("  - Contains 'QA': %v", strings.Contains(upperPrompt, "QA"))
	log.Printf("  - Contains 'REVIEW': %v", strings.Contains(upperPrompt, "REVIEW"))
	log.Printf("  - Contains 'PRIME': %v", strings.Contains(upperPrompt, "PRIME"))

	// 1. TPM Role (Planning) - Header check
	if strings.Contains(prompt, "You are an expert Technical Program Manager (TPM)") {
		log.Println("[MockAgent] Matched Role: TPM")
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

	// 2. Initializer Agent - Header check
	if strings.Contains(prompt, "## YOUR ROLE - INITIALIZER AGENT") {
		log.Println("[MockAgent] Matched Role: Initializer")
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

	// 3. QA / Manager / Reviewer - Check BEFORE Coding Agent to prevent false positives
	// The prompt might contain "Prime" in the context/report, so we must check for Manager role explicitly first.
	if strings.Contains(prompt, "## YOUR ROLE - QA AGENT") || strings.Contains(prompt, "## YOUR ROLE - PROJECT MANAGER") || strings.Contains(upperPrompt, "QA REPORT") {
		log.Println("[MockAgent] Matched Role: QA/Manager")
		return `
Looking good! The implementation meets the requirements.

`+"```bash"+`
agent-bridge signal PROJECT_SIGNED_OFF true --privileged
`+"```"+`
`, nil
	}

	// 4. Coding Agent (Implementation)
	if strings.Contains(prompt, "## YOUR ROLE - CODING AGENT") || strings.Contains(upperPrompt, "[PRIMES]") || strings.Contains(prompt, "primes.py") || strings.Contains(upperPrompt, "PRIME") {
		log.Println("[MockAgent] Matched Role: Coding Agent")
		// Python script to calculate primes < 10000
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

	// Default echo response for unknown prompts
	log.Println("[MockAgent] Fallback to default response")
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
