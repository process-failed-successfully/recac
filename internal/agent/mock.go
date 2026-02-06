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
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristic for E2E Prime Python Scenario
	// We check for feature IDs as well to be robust against description truncation or formatting changes
	if strings.Contains(prompt, "primes.py") ||
		strings.Contains(prompt, "[PRIMES]") ||
		strings.Contains(prompt, "req-implement-prime-calculation-lo") ||
		strings.Contains(prompt, "req-output-results-to-primes-json") ||
		strings.Contains(prompt, "req-verify-that-exactly-1229-prime") {
		// Detect Role to decide between Plan (JSON) and Implementation (Bash)
		if strings.Contains(prompt, "ROLE: Lead Software Architect") || strings.Contains(prompt, "ROLE - PROJECT MANAGER") {
			return m.generatePrimesArchitectPlan(), nil
		}
		if strings.Contains(prompt, "Technical Program Manager") {
			return m.generatePrimesTPMPlan(), nil
		}
		// Fallback for generic "Ticket Generation" instructions if role isn't explicit but context implies planning
		if strings.Contains(prompt, "CRITICAL INSTRUCTION FOR TICKET GENERATION") {
			// If it matches TPM signature (Epics/Stories), go TPM. Otherwise Architect.
			// But usually CRITICAL INSTRUCTION is in the spec, which is in all prompts.
			// So we rely on the specific Role headers above first.
			// If we are here, it means it has CRITICAL INSTRUCTION but NO specific role header we recognized above.
			// We'll default to Architect Plan as safe fallback for 'recac implement', unless it looks like TPM.
			return m.generatePrimesArchitectPlan(), nil
		}

		// Default to implementation if it looks like a task or coding request
		return m.generatePrimesResponse(), nil
	}

	// Return a generic mock response
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func (m *MockAgent) generatePrimesArchitectPlan() string {
	return `{
"project_name": "Prime Number Script",
"features": [
{
"id": "PRIMES",
"category": "functional",
"description": "Implement primes.py to calculate primes < 10000 and save to primes.json",
"status": "pending",
"steps": [
"Create primes.py with prime calculation logic",
"Run the script to generate primes.json",
"Verify output file exists and is valid JSON"
],
"dependencies": {
"depends_on_ids": [],
"exclusive_write_paths": ["primes.py", "primes.json"],
"read_only_paths": []
}
}
]
}`
}

func (m *MockAgent) generatePrimesTPMPlan() string {
	return `[
  {
    "title": "ID:[PRIMES] Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.",
    "type": "Task",
    "children": [],
    "acceptance_criteria": [
      "Implement prime calculation logic in primes.py",
      "Output results to primes.json",
      "Verify that exactly 1229 primes are calculated"
    ]
  }
]`
}

// SendStream implements the Agent interface
func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := m.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(resp)
	}
	return resp, err
}

func (m *MockAgent) generatePrimesResponse() string {
	return `
Here is the implementation for the prime number script.

` + "```bash" + `
# Configure git identity for CI
git config user.email "agent@recac.com"
git config user.name "Recac Agent"

# Create the python script
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

primes = [i for i in range(10000) if is_prime(i)]
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run the script to generate the output file
python3 primes.py

# Commit and push
git add primes.py primes.json
git commit -m "Implement primes.py and generate primes.json"
git push origin HEAD
` + "```" + `
`
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
