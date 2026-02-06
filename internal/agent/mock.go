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

	// Smart Mock Logic for Primes Scenario (E2E Test)
	if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "prime-python") || strings.Contains(prompt, "Prime Number Script") {
		// Distinguish between Planner (JSON) and Coding Agent (Bash)
		if strings.Contains(prompt, "Lead Software Architect") || strings.Contains(prompt, "feature_list.json") {
			return m.generatePrimesPlan(), nil
		}
		return m.generatePrimesResponse(), nil
	}

	// Default response
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

func (m *MockAgent) generatePrimesPlan() string {
	return `{
  "project_name": "Prime Number Script",
  "features": [
    {
      "id": "req-must-correctly-identify-prime-",
      "category": "functional",
      "description": "Script calculates primes correctly",
      "status": "pending",
      "steps": [
        "Step 1: Create primes.py",
        "Step 2: Implement sieve or trial division",
        "Step 3: Run script and verify primes.json output"
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

func (m *MockAgent) generatePrimesResponse() string {
	// Script to implement primes.py, run it, and commit the results
	script := `
cat << 'EOF' > primes.py
import json

def get_primes(n):
    primes = []
    for i in range(2, n):
        is_prime = True
        for j in range(2, int(i ** 0.5) + 1):
            if i % j == 0:
                is_prime = False
                break
        if is_prime:
            primes.append(i)
    return primes

primes = get_primes(10000)
with open('primes.json', 'w') as f:
    json.dump({'primes': primes}, f)
EOF

python3 primes.py
git add -f primes.py primes.json
git commit -m "Implement primes.py" || echo "No changes to commit"

# Signal completion
agent-bridge feature set req-must-correctly-identify-prime- --status done --passes true
`
	return fmt.Sprintf("Here is the solution for the Primes task:\n\n```bash\n%s\n```", script)
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
