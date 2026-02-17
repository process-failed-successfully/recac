package agent

import (
	"context"
	"fmt"
	"regexp"
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

	// Heuristic: If prompt asks for JSON tickets (TPM or ID:[PRIMES]), return JSON
	// Check "ID:[PRIMES]" but ensure it's NOT the execution prompt (which asks to create a script)
	isPlanning := strings.Contains(prompt, "Technical Program Manager")
	isPrimesPlanning := strings.Contains(prompt, "ID:[PRIMES]") && !strings.Contains(prompt, "Implement a python script named 'primes.py'")

	if isPlanning || isPrimesPlanning {
		// Extract repo URL if present to make ticket valid for JiraPoller
		repoURL := "https://github.com/example/repo"
		re := regexp.MustCompile(`(?i)Repo: (https?://\S+)`)
		matches := re.FindStringSubmatch(prompt)
		if len(matches) > 1 {
			repoURL = matches[1]
		}

		return fmt.Sprintf(`[
  {
    "title": "Implement Prime Number Generator",
    "description": "Create a Python script that generates prime numbers.\n\nREQUIRED FEATURES:\n- Function to check if a number is prime\n- Main loop to print primes\n\nRepo: %s",
    "type": "Task",
    "labels": ["recac-agent"]
  }
]`, repoURL), nil
	}

	// Heuristic: If prompt looks like a coding task (but not the TPM prompt), return a coding response
	// We want to avoid matching the TPM prompt which also contains "Python" etc.
	// We also explicitly check for the "primes.py" execution prompt
	lowerPrompt := strings.ToLower(prompt)
	isCoding := strings.Contains(lowerPrompt, "python") || strings.Contains(lowerPrompt, "go") || strings.Contains(lowerPrompt, "code") || strings.Contains(lowerPrompt, "script")
	isPrimesExecution := strings.Contains(lowerPrompt, "primes.py") && !isPlanning

	// Log the prompt for debugging CI failures
	if !isPlanning && !isCoding && !isPrimesExecution && !strings.Contains(prompt, "QA Agent") && !strings.Contains(prompt, "Manager Agent") {
		fmt.Printf("[MockAgent] Unmatched prompt: %s\n", truncateString(prompt, 200))
	}

	if (isCoding && !isPlanning) || isPrimesExecution {
		if isPrimesExecution {
			return `I will implement the prime number script as requested.

$$$
#!/bin/bash
cat << 'EOF' > primes.py
import json

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

primes = [n for n in range(10000) if is_prime(n)]

with open("primes.json", "w") as f:
    json.dump({"primes": primes}, f)
EOF

python3 primes.py
git add -f primes.json primes.py
$$$

Task completed. Primes generated.
`, nil
		}

		return `I will implement the requested changes.

$$$
#!/bin/bash
# Implementation
echo "Implementing feature..."
# Create a dummy file to satisfy verification
echo "def is_prime(n): return n > 1" > primes.py
$$$

Task completed. Tests passed.
`, nil
	}

	// Heuristic: QA Agent
	if strings.Contains(prompt, "QA Agent") {
		return "QA_PASSED", nil
	}

	// Heuristic: Manager Agent
	if strings.Contains(prompt, "Manager Agent") {
		return "PROJECT_SIGNED_OFF", nil
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
