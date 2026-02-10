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

	// Heuristics for CI Smoke Tests
	// 1. TPM Agent (Jira Ticket Generation)
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "generate a JSON list of Jira tickets") {
		return `[
  {
    "title": "ID:[PRIMES] Implement Prime Number Script",
    "description": "Create a Python script that calculates prime numbers.",
    "type": "Task",
    "labels": ["recac-agent"]
  },
  {
    "title": "ID:[PRIMES] Verify Output",
    "description": "Ensure the script outputs the correct number of primes.",
    "type": "Task",
    "labels": ["recac-agent"]
  }
]`, nil
	}

	// 2. Initializer Agent (Setup)
	if strings.Contains(prompt, "INITIALIZER AGENT") {
		return "```bash\necho 'Initializing environment...'\n" +
			"cat <<EOF > feature_list.json\n" +
			"{\n" +
			"  \"project_name\": \"Mock Project\",\n" +
			"  \"features\": [\n" +
			"    {\n" +
			"      \"id\": \"1\",\n" +
			"      \"category\": \"Core\",\n" +
			"      \"priority\": \"High\",\n" +
			"      \"description\": \"Implement Prime Number Script (python)\",\n" +
			"      \"status\": \"todo\",\n" +
			"      \"passes\": false\n" +
			"    }\n" +
			"  ]\n" +
			"}\nEOF\n```", nil
	}

	// 3. Coding Agent (Implementation)
	if strings.Contains(prompt, "CODING AGENT") || (strings.Contains(prompt, "prime") && strings.Contains(prompt, "python")) {
		return "```bash\n" +
			"cat <<EOF > primes.py\n" +
			"import json\n\n" +
			"def is_prime(n):\n" +
			"    if n < 2: return False\n" +
			"    for i in range(2, int(n**0.5) + 1):\n" +
			"        if n % i == 0: return False\n" +
			"    return True\n\n" +
			"primes = [x for x in range(10000) if is_prime(x)]\n" +
			"print(f\"Found {len(primes)} primes\")\n\n" +
			"with open(\"primes.json\", \"w\") as f:\n" +
			"    json.dump({\"primes\": primes}, f)\n" +
			"EOF\n\n" +
			"# Run the script\n" +
			"python3 primes.py\n\n" +
			"# Commit results\n" +
			"git config --global user.email \"agent@recac.ai\"\n" +
			"git config --global user.name \"Recac Agent\"\n" +
			"git add primes.py primes.json\n" +
			"git commit -m \"Add primes script and output\" || echo \"Nothing to commit\"\n" +
			"git push\n" +
			"cat <<EOF > feature_list.json\n" +
			"{\n" +
			"  \"project_name\": \"Mock Project\",\n" +
			"  \"features\": [\n" +
			"    {\n" +
			"      \"id\": \"1\",\n" +
			"      \"category\": \"Core\",\n" +
			"      \"priority\": \"High\",\n" +
			"      \"description\": \"Implement Prime Number Script (python)\",\n" +
			"      \"status\": \"done\",\n" +
			"      \"passes\": true\n" +
			"    }\n" +
			"  ]\n" +
			"}\nEOF\n" +
			"```", nil
	}

	// 4. QA Agent
	if strings.Contains(prompt, "QA AGENT") {
		return "```bash\n" +
			"echo \"Running QA checks...\"\n" +
			"# Signal completion\n" +
			"agent-bridge signal --privileged QA_PASSED true\n" +
			"agent-bridge signal --privileged PROJECT_SIGNED_OFF true\n" +
			"```", nil
	}

	// Default response
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
