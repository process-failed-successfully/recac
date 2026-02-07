package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a smart mock agent for testing and mock mode
// It returns predefined responses based on heuristics to pass E2E tests
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

	// 1. Heuristic: Technical Program Manager (TPM) - Planning Phase
	// Checks for role definition or critical instruction from prime_python.go
	if (strings.Contains(prompt, "Technical Program Manager") ||
		strings.Contains(prompt, "CRITICAL INSTRUCTION FOR TICKET GENERATION")) &&
		!strings.Contains(prompt, "YOUR ROLE - CODING AGENT") {

		return `[
  {
    "id": "PRIMES",
    "type": "Task",
    "title": "[PRIMES] Create Prime Number Script",
    "description": "Implement primes.py to calculate primes < 10000 and output to primes.json. Verify 1229 primes."
  }
]`, nil
	}

	// 2. Heuristic: Developer - Implementation Phase for [PRIMES]
	// Checks for ticket ID or specific file requirement
	// 4. Heuristic: Initializer - Setup Phase
	// Checks for the initializer role header
	if strings.Contains(prompt, "YOUR ROLE - INITIALIZER AGENT") {
		// Use RECAC_PROJECT_ID if available for the project name
		// Use full path to agent-bridge to be safe
		return "```bash\n#!/bin/bash\n" +
			"cat <<EOF | /usr/local/bin/agent-bridge import\n" +
			"{\n" +
			"  \"project_name\": \"${RECAC_PROJECT_ID:-MFLP-7063}\",\n" +
			"  \"features\": [\n" +
			"    {\n" +
			"      \"id\": \"req-primes-py-exists\",\n" +
			"      \"category\": \"core\",\n" +
			"      \"priority\": \"high\",\n" +
			"      \"description\": \"Implement primes.py to calculate primes < 10000 and output to primes.json\",\n" +
			"      \"status\": \"todo\",\n" +
			"      \"dependencies\": []\n" +
			"    }\n" +
			"  ]\n" +
			"}\n" +
			"EOF\n" +
			"```", nil
	}

	// 2. Heuristic: Developer - Implementation Phase for [PRIMES]
	// Checks for ticket ID or specific file requirement
	if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "primes.py") {
		return `
` + "```bash" + `
#!/bin/bash
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [p for p in range(10000) if is_prime(p)]

with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)
EOF

# Run the script to generate the output
python3 primes.py

# Git configuration for CI
git config --global user.email "bot@recac.com"
git config --global user.name "Recac Bot"

# Track and commit files
git add -f primes.py primes.json
git diff --quiet --staged || git commit -m "Implement primes.py and generate output"
` + "```" + `
`, nil
	}

	// 3. Fallback: Generic Response
	// Return a mock response that shows the agent received the prompt
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\necho 'Processing request...'\n\nPrompt preview: %s...",
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
