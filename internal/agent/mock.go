package agent

import (
	"context"
	"fmt"
	"strings"
)

// MockAgent is a smart mock agent for testing and mock mode.
// It uses heuristics to determine the persona (Initializer, TPM, Coding, QA, Manager)
// and returns appropriate canned responses to simulate a full workflow.
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
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	promptUpper := strings.ToUpper(prompt)

	// 1. Initializer Agent
	// Triggers: "INITIALIZER AGENT", "YOU ARE THE INITIALIZER"
	if strings.Contains(promptUpper, "INITIALIZER AGENT") || strings.Contains(promptUpper, "YOU ARE THE INITIALIZER") {
		return "```bash\n" +
			"# Initializer Setup\n" +
			"echo \"Setting up environment...\"\n" +
			"touch .env\n\n" +
			"# Create feature list in DB via agent-bridge\n" +
			"cat << 'EOF' | agent-bridge import --project \"${RECAC_PROJECT_ID}\"\n" +
			"{\n" +
			"  \"project_name\": \"Test Project\",\n" +
			"  \"features\": [\n" +
			"    {\n" +
			"      \"id\": \"req-primes\",\n" +
			"      \"description\": \"Implement prime number generator\",\n" +
			"      \"priority\": \"MVP\",\n" +
			"      \"status\": \"pending\",\n" +
			"      \"steps\": [\"Run python3 primes.py\", \"Check output\"],\n" +
			"      \"dependencies\": {\"exclusive_write_paths\": [\"primes.py\"]}\n" +
			"    }\n" +
			"  ]\n" +
			"}\n" +
			"EOF\n" +
			"```", nil
	}

	// 2. Technical Project Manager (TPM)
	// Triggers: "TECHNICAL PROJECT MANAGER", "TPM", "CREATE A PLAN"
	// Note: Must check this BEFORE generic "Manager" check if they overlap, but "Technical" is specific.
	if strings.Contains(promptUpper, "TECHNICAL PROJECT MANAGER") || strings.Contains(promptUpper, "TPM AGENT") {
		// Return JSON plan
		return "```json\n" +
			"{\n" +
			"  \"project_name\": \"Test Project\",\n" +
			"  \"features\": [\n" +
			"    {\n" +
			"      \"id\": \"req-primes\",\n" +
			"      \"description\": \"Implement prime number generator\",\n" +
			"      \"priority\": \"MVP\",\n" +
			"      \"status\": \"pending\",\n" +
			"      \"steps\": [\"Run python3 primes.py\", \"Check output\"],\n" +
			"      \"dependencies\": {\"exclusive_write_paths\": [\"primes.py\"]}\n" +
			"    }\n" +
			"  ]\n" +
			"}\n" +
			"```", nil
	}

	// 3. QA Agent
	// Triggers: "QA AGENT", "QUALITY ASSURANCE"
	if strings.Contains(promptUpper, "QA AGENT") || strings.Contains(promptUpper, "QUALITY ASSURANCE") {
		return "Run tests and signal success.\n" +
			"```bash\n" +
			"echo \"Running tests...\"\n" +
			"python3 primes.py || echo \"No tests found, skipping\"\n" +
			"agent-bridge signal --privileged QA_PASSED true\n" +
			"```", nil
	}

	// 4. Project Manager / Reviewer (Final Sign-off)
	// Triggers: "PROJECT MANAGER", "MANAGER", "REVIEW"
	if strings.Contains(promptUpper, "PROJECT MANAGER") || strings.Contains(promptUpper, "MANAGER") || strings.Contains(promptUpper, "REVIEW") {
		return "Project Approved.\n\n" +
			"```bash\n" +
			"agent-bridge signal --privileged PROJECT_SIGNED_OFF true\n" +
			"```", nil
	}

	// 5. Coding Agent (Default if role is mentioned)
	// Triggers: "CODING AGENT", "DEVELOPER", "WRITE CODE"
	if strings.Contains(promptUpper, "CODING AGENT") || strings.Contains(promptUpper, "DEVELOPER") {
		// Detect task from prompt context if possible, otherwise generic prime implementation
		if strings.Contains(promptUpper, "PRIME") || strings.Contains(promptUpper, "PRIMES") {
			return "Implementing primes.py.\n" +
				"```python:primes.py\n" +
				"import sys\n\n" +
				"def is_prime(n):\n" +
				"    if n <= 1: return False\n" +
				"    for i in range(2, int(n**0.5) + 1):\n" +
				"        if n % i == 0: return False\n" +
				"    return True\n\n" +
				"if __name__ == \"__main__\":\n" +
				"    print(\"Primes up to 20:\")\n" +
				"    for i in range(20):\n" +
				"        if is_prime(i):\n" +
				"            print(i)\n" +
				"```", nil
		}
		// Generic code if no specific task detected
		return "I am the coding agent. I will implement the requested feature.\n" +
			"```bash\n" +
			"echo \"Coding task complete\"\n" +
			"```", nil
	}

	// Fallback: Return a mock response that shows the agent received the prompt
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
