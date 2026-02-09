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

	// === Heuristics for Smoke Tests ===

	// 1. Initializer / Feature List Generation
	if strings.Contains(prompt, "CREATE FEATURE_LIST.JSON") {
		return "```bash\n" +
			"cat <<EOF > feature_list.json\n" +
			"[\n" +
			"  {\n" +
			"    \"id\": \"req-script-prints-primes\",\n" +
			"    \"title\": \"Implement Primes Script\",\n" +
			"    \"description\": \"Create a python script that prints primes.\",\n" +
			"    \"status\": \"pending\",\n" +
			"    \"type\": \"Task\"\n" +
			"  }\n" +
			"]\n" +
			"EOF\n" +
			"agent-bridge import < feature_list.json\n" +
			"```", nil
	}

	// 2. Loop Breaker (Clean State)
	if strings.Contains(strings.ToLower(prompt), "nothing to commit") ||
	   strings.Contains(strings.ToLower(prompt), "working tree clean") {
		return "```bash\n" +
			"agent-bridge signal --privileged QA_PASSED true\n" +
			"agent-bridge signal --privileged PROJECT_SIGNED_OFF true\n" +
			"```", nil
	}

	// 3. Technical Program Manager (TPM) - Jira Ticket Generation
	// Must return valid JSON list of tickets
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") {
		// Extract Repo URL if available to make it realistic
		repoRegex := regexp.MustCompile(`Repo: (https?://\S+)`)
		match := repoRegex.FindStringSubmatch(prompt)
		repoUrl := "https://example.com/repo"
		if len(match) > 1 {
			repoUrl = match[1]
		}

		// Note: Returns title (not summary) as per memory requirements for internal/jira/client.go mapping
		return fmt.Sprintf(`[
  {
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Implement a Python script to generate prime numbers.\n\nRepository: %s\n\nAcceptance Criteria:\n- req-script-prints-primes-up-to-100: Script prints primes up to 100",
    "type": "Task",
    "status": "To Do",
    "priority": "High",
    "labels": ["backend", "python"]
  }
]`, repoUrl), nil
	}

	// 4. Coding Agent - Primes Scenario
	if (strings.Contains(prompt, "CODING AGENT") || strings.Contains(prompt, "YOUR ROLE")) &&
	   (strings.Contains(prompt, "prime") || strings.Contains(prompt, "req-script-prints-primes")) {
		return "```bash\n" +
			"cat <<EOF > primes.py\n" +
			"import json\n" +
			"\n" +
			"def is_prime(n):\n" +
			"    if n <= 1:\n" +
			"        return False\n" +
			"    for i in range(2, int(n**0.5) + 1):\n" +
			"        if n % i == 0:\n" +
			"            return False\n" +
			"    return True\n" +
			"\n" +
			"primes = [p for p in range(2, 101) if is_prime(p)]\n" +
			"print(json.dumps({\"primes\": primes}))\n" +
			"EOF\n" +
			"\n" +
			"python3 primes.py\n" +
			"agent-bridge feature set req-script-prints-primes passed || true\n" +
			"git add primes.py\n" +
			"git commit -m \"Implement primes.py\" || echo \"Nothing to commit\"\n" +
			"```", nil
	}

	// 5. QA Agent
	if strings.Contains(prompt, "QA AGENT") {
		return "```bash\n" +
			"python3 primes.py\n" +
			"agent-bridge signal --privileged QA_PASSED true\n" +
			"```", nil
	}

	// 6. Project Manager Review
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return "```bash\n" +
			"agent-bridge signal --privileged PROJECT_SIGNED_OFF true\n" +
			"```", nil
	}

	// Fallback for debugging
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
