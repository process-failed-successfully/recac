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

	// Heuristics for smoke tests

	// 1. Coding Task (Execution Phase) - Check FIRST to avoid false positives from history
	// Trigger: "YOUR ROLE - CODING AGENT" (high confidence) OR "prime"/"primes.json"/"ID:[PRIMES]"
	// The TPM prompt might also contain "prime", so we prioritize the role check.
	isCodingPhase := strings.Contains(prompt, "YOUR ROLE - CODING AGENT")
	isPrimesTask := strings.Contains(prompt, "prime") ||
		strings.Contains(prompt, "primes.json") ||
		strings.Contains(prompt, "ID:[PRIMES]") ||
		strings.Contains(prompt, "Script accepts a limit") ||
		strings.Contains(prompt, "unit tests")

	if isCodingPhase && isPrimesTask {
		return m.primesImplementation(prompt), nil
	}

	// Fallback for simple tests without the full prompt template
	if !strings.Contains(prompt, "Technical Program Manager") && isPrimesTask {
		return m.primesImplementation(prompt), nil
	}

	// 2. Ticket Generation (Planning Phase)
	// Trigger: "Technical Program Manager" or "TPM" or "ticket generation"
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") || strings.Contains(prompt, "ticket generation") {
		return `[
  {
    "title": "ID:[PRIMES] Implement prime number generator",
    "description": "Create a Python script that generates prime numbers up to a specified limit. The script should be efficient and well-documented.\n\nRepo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Task",
    "priority": "High",
    "labels": ["backend", "python"],
    "acceptance_criteria": [
      "Script accepts a limit as a command-line argument",
      "Outputs prime numbers to stdout",
      "Includes unit tests"
    ]
  }
]`, nil
	}

	// 3. QA Agent
	if strings.Contains(prompt, "QA AGENT") || strings.Contains(prompt, "QA Agent") {
		return "QA Checks Passed.\n\n```bash\nagent-bridge signal QA_PASSED true\n```", nil
	}

	// 4. Manager Agent
	if strings.Contains(prompt, "Manager Agent") || strings.Contains(prompt, "MANAGER AGENT") {
		return "Project Approved.\n\n```bash\nagent-bridge signal PROJECT_SIGNED_OFF true\n```", nil
	}

	// Default Mock Response
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100))
	return response, nil
}

func (m *MockAgent) primesImplementation(prompt string) string {
	// Try to extract the Feature ID to mark it as done
	// Pattern matches "**Feature ID**: {id}"
	re := regexp.MustCompile(`\*\*Feature ID\*\*: ([\w-]+)`)
	matches := re.FindStringSubmatch(prompt)
	featureID := ""
	if len(matches) > 1 {
		featureID = matches[1]
	}

	script := `I will implement the prime number generator in Python.

` + "```bash" + `
cat << 'EOF' > primes.py
import sys

def is_prime(n):
    if n <= 1:
        return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0:
            return False
    return True

def generate_primes(limit):
    primes = []
    for num in range(2, limit + 1):
        if is_prime(num):
            primes.append(num)
    return primes

if __name__ == "__main__":
    limit = 100
    if len(sys.argv) > 1:
        try:
            limit = int(sys.argv[1])
        except ValueError:
            pass

    primes = generate_primes(limit)
    print(primes)
EOF
` + "```" + `

And I'll run it to verify:

` + "```bash" + `
python3 primes.py 50
` + "```"

	// If we found a feature ID, mark it as done
	if featureID != "" {
		script += fmt.Sprintf("\n\nAnd mark the feature as complete:\n\n```bash\nagent-bridge feature set %s --status done --passes true\n```", featureID)
	}

	return script
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
