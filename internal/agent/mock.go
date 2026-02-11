package agent

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
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
// It returns a mock response based on the role detected in the prompt
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// === Heuristic Detection for Roles ===

	// 1. Technical Program Manager (TPM) - Planning Phase
	// The TPM prompt does NOT start with ## YOUR ROLE, but "You are an expert Technical Program Manager..."
	if strings.Contains(prompt, "## YOUR ROLE - TECHNICAL PROGRAM MANAGER") ||
	   strings.Contains(prompt, "You are an expert Technical Program Manager (TPM)") ||
	   strings.Contains(prompt, "Analyze the user's request and create a detailed plan") {

		// Determine Project ID
		projectID := os.Getenv("RECAC_PROJECT_ID")
		if projectID == "" {
			// Try to find ID in prompt
			re := regexp.MustCompile(`ID:\[(.*?)\]`)
			matches := re.FindStringSubmatch(prompt)
			if len(matches) > 1 {
				projectID = matches[1]
			} else {
				projectID = "PRIMES"
			}
		}

		// Return a mock ticket list (Array of ticketNode) for the Prime Number Script scenario
		// This matches []ticketNode expected by recac jira generate-from-spec
		return fmt.Sprintf("```json\n[\n  {\n    \"title\": \"ID:[%s] Prime Number Project\",\n    \"description\": \"Implement a Python script to calculate prime numbers. Repo: https://github.com/example/repo\",\n    \"type\": \"Epic\",\n    \"children\": [\n      {\n        \"title\": \"ID:[req-script-runs] Implement primes.py\",\n        \"description\": \"Create the main script. Repo: https://github.com/example/repo\",\n        \"type\": \"Story\",\n        \"acceptance_criteria\": [\n          \"Script runs without errors\",\n          \"Calculates first n primes correctly\"\n        ]\n      }\n    ]\n  }\n]\n```", projectID), nil
	}

	// 2. Initializer Agent
	if strings.Contains(prompt, "## YOUR ROLE - INITIALIZER AGENT") {
		// Initializer must import features via agent-bridge
		return `I will initialize the project.

` + "```bash" + `
cat << 'EOF' | agent-bridge import
{
  "project_name": "Prime Number Script",
  "features": [
    {
      "id": "req-script-runs",
      "description": "Implement primes.py",
      "status": "pending",
      "priority": "MVP"
    }
  ]
}
EOF

cat << 'EOF' > init.sh
#!/bin/bash
echo "Initializing..."
EOF
chmod +x init.sh
` + "```" + `
`, nil
	}

	// 3. Coding Agent (Developer) - Implementation Phase
	if strings.Contains(prompt, "## YOUR ROLE - CODING AGENT") ||
	   strings.Contains(prompt, "You are an expert software engineer") {

		// If tasked with primes.py (common smoke test scenario)
		if strings.Contains(prompt, "prime") || strings.Contains(prompt, "primes.py") {
			// Return python script implementation
			script := `
def is_prime(n):
    if n <= 1: return False
    for i in range(2, int(n**0.5) + 1):
        if n % i == 0: return False
    return True

def get_primes(n):
    primes = []
    i = 2
    while len(primes) < n:
        if is_prime(i):
            primes.append(i)
        i += 1
    return primes

if __name__ == "__main__":
    import sys
    n = int(sys.argv[1]) if len(sys.argv) > 1 else 10
    print(f"First {n} primes: {get_primes(n)}")
`
			// Escape % for Sprintf if needed, but here we just return raw string
			// We return a block that writes the file and runs it to show progress
			return fmt.Sprintf("I will implement the prime number script.\n\n```bash\ncat <<EOF > primes.py%sEOF\n\npython3 primes.py 5\n```\n\nI have implemented the script and verified it works.", script), nil
		}
	}

	// 4. QA Agent - Verification Phase
	if strings.Contains(prompt, "## YOUR ROLE - QA AGENT") {
		// Always pass for smoke tests
		return "Based on my analysis, the code implements the requirements correctly.\n\nQA Status: PASS\n\nExisting Issues: None\n\n```bash\nagent-bridge signal QA_PASSED true\n```", nil
	}

	// 5. Project Manager - Review Phase
	if strings.Contains(prompt, "## YOUR ROLE - PROJECT MANAGER") {
		// Approve and sign off
		return "The project looks good. All features are implemented and QA passed.\n\nDecision: APPROVED\n\nNext Steps: Release\n\n```bash\nagent-bridge signal PROJECT_SIGNED_OFF true\n```", nil
	}

	// Default Fallback
	return fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100)), nil
}

// SendStream implements the Agent interface
func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	// Simulate processing delay for realism
	time.Sleep(10 * time.Millisecond) // Reduced from 100ms to 10ms to speed up tests

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
