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

	fmt.Printf("DEBUG: MockAgent Prompt: %s\n", truncateString(prompt, 100))

	// 1. Initializer Agent
	if strings.Contains(prompt, "ROLE - INITIALIZER AGENT") {
		jsonContent := `{"projectName":"mock-project","features":[{"name":"Core","description":"Initial feature","status":"todo"}]}`
		if strings.Contains(prompt, "primes") || strings.Contains(prompt, "PRIMES") || strings.Contains(prompt, "prime-python") {
			jsonContent = `{"projectName":"prime-python","features":[{"name":"Implement Primes","description":"Create a python script named primes.py to calculate prime numbers.","status":"todo","priority":"high"}]}`
		}

		return fmt.Sprintf(`%s:

I am the Initializer Agent. I am setting up the environment.

`+"```bash"+`
echo '%s' > feature_list.json
echo "Initializer Agent Setup Complete"
`+"```"+`
`, m.responsePrefix, jsonContent), nil
	}

	// 2. TPM Agent / Project Manager (Ticket Generation)
	// Note: We check for "Technical Program Manager" specifically for the planning phase.
	if strings.Contains(prompt, "Technical Program Manager") {
		// Return JSON list of tickets
		// Check for specific scenario [PRIMES]
		if strings.Contains(prompt, "prime-python") || strings.Contains(prompt, "PRIMES") || strings.Contains(prompt, "primes") {
			return `[
  {
    "id": "TASK-1",
    "title": "ID:[PRIMES] Implement Primes",
    "description": "Implement a python script named primes.py to calculate prime numbers.",
    "type": "task",
    "status": "todo",
    "priority": "high"
  }
]`, nil
		}
		// Default tickets
		return `[
  {
    "id": "TASK-1",
    "title": "Generic Task",
    "description": "A generic task for testing.",
    "type": "task",
    "status": "todo"
  }
]`, nil
	}

	// 3. Architect Agent (Feature List)
	if strings.Contains(prompt, "Lead Software Architect") {
		return `{
  "features": [
    {
      "name": "Core Feature",
      "description": "The core feature of the application.",
      "status": "todo",
      "priority": "high"
    }
  ]
}`, nil
	}

	// 4. Coding Agent (Implementation)
	// Check for primes task specifically
	if strings.Contains(prompt, "primes.py") {
		return `Here is the implementation for primes.py:

` + "```bash" + `
cat << 'EOF' > primes.py
import sys
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

if __name__ == "__main__":
    limit = 10000
    primes = [i for i in range(limit) if is_prime(i)]
    with open("primes.json", "w") as f:
        json.dump({"primes": primes}, f)
    print(f"Generated {len(primes)} primes up to {limit}")
EOF

python3 primes.py

git add primes.py primes.json
git commit -m "Add primes.py and primes.json" || echo "No changes to commit"
git push
` + "```" + `
`, nil
	}

	// 5. QA Agent / Project Manager (Review/Approval)
	if strings.Contains(prompt, "ROLE - PROJECT MANAGER") {
		return `The project looks good.

APPROVED`, nil
	}

	// 6. Default / Fallback
	// Log unmatched
	fmt.Printf("[MockAgent] UNMATCHED PROMPT: %s\n", truncateString(prompt, 50))

	// Return a response with a dummy command to prevent NO-OP loops
	return fmt.Sprintf(`%s:

I received your prompt. Here is a command to verify I am working:

`+"```bash"+`
echo "Mock Agent Default Response"
`+"```"+`

Prompt preview: %s...`,
		m.responsePrefix, truncateString(prompt, 100)), nil
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
