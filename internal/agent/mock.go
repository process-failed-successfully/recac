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

	// 1. Initializer / Architect Role (Creating Tickets)
	// Detect based on keywords from PrimePythonScenario.AppSpec or TPM prompts
	// We need to be careful not to match the developer prompt which might also contain ID:[PRIMES]
	// The Initializer/TPM prompt typically asks to "Create exactly ONE ticket", contains "feature_list.json" context,
	// or identifies as "Technical Program Manager".
	// We check for these strong signals first. Even if the prompt contains "implement" (e.g. in the spec description),
	// we should prioritize the ticket creation role if the explicit instruction is present.
	if strings.Contains(prompt, "Create exactly ONE ticket") ||
		strings.Contains(prompt, "feature_list.json") ||
		strings.Contains(prompt, "Technical Program Manager") {
		// Return a JSON list of tickets as expected by the Jira/Spec parser
		return `
[
  {
    "title": "ID:[PRIMES] Implement Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.",
    "type": "Task"
  }
]
`, nil
	}

	// 2. Developer Role (Implementation)
	// Detect request to implement the script. The runner usually sends the ticket description.
	// The prompt will contain the task description "Implement a python script...".
	if strings.Contains(prompt, "Implement a python script") || (strings.Contains(prompt, "primes.py") && !strings.Contains(prompt, "Review") && !strings.Contains(prompt, "QA")) {
		return `
I will implement the prime number script as requested.

` + "```bash" + `
# Create the python script
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n < 2:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

# Calculate primes < 10000
primes = [i for i in range(10000) if is_prime(i)]

# Output to JSON
with open('primes.json', 'w') as f:
    json.dump({"primes": primes}, f)
EOF

# Run the script to generate the output file
python3 primes.py

# Commit the changes
git add primes.py primes.json
git commit -m "feat: implement primes.py and generate primes.json"
git push || echo "Push skipped in mock mode"
` + "```" + `
`, nil
	}

	// 3. QA / Manager Role (Review)
	if strings.Contains(prompt, "Review") || strings.Contains(prompt, "QA") {
		return "LGTM. The code implements the requirements correctly. primes.py exists and primes.json contains the expected data.", nil
	}

	// Default fallback
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
