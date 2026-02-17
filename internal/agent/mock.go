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
	// Debug logging
	fmt.Printf("[MockAgent] Received Prompt (len=%d): %s\n", len(prompt), truncateString(prompt, 200))

	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Detect "TPM" / Ticket Generation request
	if strings.Contains(prompt, "Technical Program Manager") {
		if strings.Contains(prompt, "ID:[PRIMES]") || strings.Contains(prompt, "prime") {
			return `[
  {
    "id": "ID:[PRIMES]",
    "title": "ID:[PRIMES] Implement Prime Number Generator",
    "description": "Implement a Python script to generate prime numbers up to 10,000.",
    "type": "Task",
    "dependencies": []
  }
]`, nil
		}
	}

	// Detect "INITIALIZER AGENT" request
	// This agent must generate feature_list.json to allow the session to proceed.
	if strings.Contains(prompt, "INITIALIZER AGENT") {
		// Create a feature list that matches the prime task
		return "```bash\n" +
			"cat <<EOF > feature_list.json\n" +
			"{\n" +
			"  \"features\": [\n" +
			"    {\n" +
			"      \"id\": \"req-1\",\n" +
			"      \"description\": \"Implement Prime Number Generator\",\n" +
			"      \"status\": \"todo\",\n" +
			"      \"passes\": false\n" +
			"    }\n" +
			"  ]\n" +
			"}\n" +
			"EOF\n" +
			"```", nil
	}

	// Detect "QA Agent" request
	if strings.Contains(prompt, "QA Agent") {
		return "```bash\necho 'QA_PASSED'\n```", nil
	}

	// Detect "Manager Agent" request
	// This must be checked BEFORE the generic "Success" check, because the Manager prompt might contain "Success"
	// or "passed" from the QA report/history, but we want the Manager action (Sign Off).
	if strings.Contains(prompt, "Manager Agent") || strings.Contains(prompt, "PROJECT MANAGER") {
		return "```bash\nagent-bridge signal --privileged PROJECT_SIGNED_OFF\n```", nil
	}

	// Detect successful execution or test completion to stop the loop
	// We check for "Success:" combined with the file we expect, or standard test output
	// This is primarily for the Coding Agent to recognize it's done.
	if (strings.Contains(prompt, "Success:") && (strings.Contains(prompt, "primes.py") || strings.Contains(prompt, "test"))) ||
		strings.Contains(prompt, "ran 2 tests") ||
		strings.Contains(prompt, "ok") {
		fmt.Println("[MockAgent] Detected completion signal. Returning 'Task completed'.")
		return "Task completed. The implementation and verification are successful.", nil
	}

	// Detect "CODING AGENT" request
	// We explicitly exclude "Technical Program Manager" to avoid misfiring on the planning prompt if it contains "Implement"
	if (strings.Contains(prompt, "CODING AGENT") || strings.Contains(prompt, "Implement") || strings.Contains(prompt, "write")) &&
		!strings.Contains(strings.ToLower(prompt), "technical program manager") {
		// Return a bash script to implement the prime generator
		// We use a simple script that passes the verification
		return "```bash\n" +
			"cat <<EOF > primes.py\n" +
			"import json\n" +
			"\n" +
			"def is_prime(n):\n" +
			"    if n < 2:\n" +
			"        return False\n" +
			"    for i in range(2, int(n**0.5) + 1):\n" +
			"        if n % i == 0:\n" +
			"            return False\n" +
			"    return True\n" +
			"\n" +
			"primes = [i for i in range(10000) if is_prime(i)]\n" +
			"print(json.dumps({\"primes\": primes}))\n" +
			"EOF\n" +
			"python3 primes.py\n" +
			"```", nil
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
