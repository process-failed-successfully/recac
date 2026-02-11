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
	// The prompt usually contains "## YOUR ROLE - <ROLE NAME>"

	// 1. Technical Program Manager (TPM) - Planning Phase
	if strings.Contains(prompt, "## YOUR ROLE - TECHNICAL PROGRAM MANAGER") ||
	   strings.Contains(prompt, "Analyze the user's request and create a detailed plan") {

		// If prompt asks for JSON plan, return valid JSON feature list
		if strings.Contains(prompt, "JSON") {
			// Extract project ID if possible, default to "PRIMES" for smoke test
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

			// Return a mock feature list for the Prime Number Script scenario
			return fmt.Sprintf("```json\n{\n  \"project_name\": \"%s\",\n  \"features\": [\n    {\n      \"id\": \"1\",\n      \"description\": \"Implement a Python script (primes.py) that calculates the first n prime numbers.\",\n      \"priority\": \"high\",\n      \"status\": \"todo\"\n    },\n    {\n      \"id\": \"2\",\n      \"description\": \"Add unit tests for the prime calculation logic.\",\n      \"priority\": \"medium\",\n      \"status\": \"todo\"\n    }\n  ]\n}\n```", projectID), nil
		}
	}

	// 2. Coding Agent (Developer) - Implementation Phase
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

	// 3. QA Agent - Verification Phase
	if strings.Contains(prompt, "## YOUR ROLE - QA AGENT") {
		// Always pass for smoke tests
		return "Based on my analysis, the code implements the requirements correctly.\n\nQA Status: PASS\n\nExisting Issues: None", nil
	}

	// 4. Project Manager - Review Phase
	if strings.Contains(prompt, "## YOUR ROLE - PROJECT MANAGER") {
		// Approve and sign off
		return "The project looks good. All features are implemented and QA passed.\n\nDecision: APPROVED\n\nNext Steps: Release", nil
	}

	// Default Fallback
	return fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...",
		m.responsePrefix, len(prompt), truncateString(prompt, 100)), nil
}

// SendStream implements the Agent interface
func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	// Simulate processing delay for realism
	time.Sleep(100 * time.Millisecond)

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
