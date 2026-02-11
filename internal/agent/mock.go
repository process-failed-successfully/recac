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

	// Heuristic Role Detection for Smoke Tests

	// 1. Technical Program Manager (Planning Phase)
	if strings.Contains(prompt, "You are an expert Technical Program Manager") {
		// Return JSON plan for tickets
		return `[
  {
    "title": "ID:[PRIMES] Implement Primes",
    "description": "Implement a Python script to calculate prime numbers.",
    "type": "Task",
    "status": "todo",
    "children": []
  }
]`, nil
	}

	// 2. Initializer Agent (Feature Import)
	if strings.Contains(prompt, "## YOUR ROLE - INITIALIZER AGENT") {
		// Return bash script to import features
		// We use cat EOF to avoid escaping issues
		return "```bash\n" +
			"cat << 'EOF' > feature_list.json\n" +
			"{\n" +
			"  \"project_name\": \"MFLP-11007\",\n" +
			"  \"features\": [\n" +
			"    {\n" +
			"      \"id\": \"PRIMES\",\n" +
			"      \"description\": \"Implement primes.py\",\n" +
			"      \"status\": \"todo\",\n" +
			"      \"files\": [\"primes.py\"]\n" +
			"    }\n" +
			"  ]\n" +
			"}\n" +
			"EOF\n\n" +
			"# Import into DB using agent-bridge\n" +
			"agent-bridge import feature_list.json\n" +
			"```", nil
	}

	// 3. Coding Agent (Implementation)
	if strings.Contains(prompt, "## YOUR ROLE - CODING AGENT") {
		// Check if it's the Primes task
		if strings.Contains(prompt, "PRIMES") || strings.Contains(prompt, "primes.py") {
			return "```bash\n" +
				"# Implement primes.py\n" +
				"cat << 'EOF' > primes.py\n" +
				"def is_prime(n):\n" +
				"    if n <= 1: return False\n" +
				"    for i in range(2, int(n**0.5) + 1):\n" +
				"        if n % i == 0: return False\n" +
				"    return True\n\n" +
				"if __name__ == '__main__':\n" +
				"    import sys\n" +
				"    # Print primes up to 20\n" +
				"    print([x for x in range(20) if is_prime(x)])\n" +
				"EOF\n\n" +
				"# Run it to verify\n" +
				"python3 primes.py\n\n" +
				"# Mark feature as completed\n" +
				"agent-bridge feature update PRIMES --status completed\n" +
				"```", nil
		}
	}

	// 4. QA Agent (Verification)
	if strings.Contains(prompt, "## YOUR ROLE - QA AGENT") {
		// Signal success
		return "QA Passed. All tests look good.\n\nSIGNAL: QA_PASSED", nil
	}

	// 5. Project Manager (Sign-off)
	if strings.Contains(prompt, "## YOUR ROLE - PROJECT MANAGER") {
		return "Approved. Great work.", nil
	}

	// Default Fallback
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
