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
// It returns a mock response based on heuristics or forced response
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	lowerPrompt := strings.ToLower(prompt)

	// 1. Initializer Agent
	if strings.Contains(lowerPrompt, "initializer agent") || (strings.Contains(lowerPrompt, "initialize") && strings.Contains(lowerPrompt, "feature")) {
		return "```bash\n" +
			"cat <<EOF > feature_list.json\n" +
			"{\n" +
			"  \"project_name\": \"MFLP-12559\",\n" +
			"  \"features\": [\n" +
			"    {\n" +
			"      \"id\": \"req-the-script-primes-py-is-implem\",\n" +
			"      \"category\": \"functional\",\n" +
			"      \"priority\": \"critical\",\n" +
			"      \"description\": \"The script 'primes.py' is implemented and calculates all prime numbers less than 10,000\",\n" +
			"      \"status\": \"pending\",\n" +
			"      \"passes\": false,\n" +
			"      \"steps\": null,\n" +
			"      \"dependencies\": null\n" +
			"    },\n" +
			"    {\n" +
			"      \"id\": \"req-the-output-is-written-to-a-fil\",\n" +
			"      \"category\": \"functional\",\n" +
			"      \"priority\": \"critical\",\n" +
			"      \"description\": \"The output is written to a file named 'primes.json'\",\n" +
			"      \"status\": \"pending\",\n" +
			"      \"passes\": false,\n" +
			"      \"steps\": null,\n" +
			"      \"dependencies\": null\n" +
			"    },\n" +
			"    {\n" +
			"      \"id\": \"req-the-primes-json-file-contains-\",\n" +
			"      \"category\": \"functional\",\n" +
			"      \"priority\": \"critical\",\n" +
			"      \"description\": \"The 'primes.json' file contains a single key 'primes' with a list of integers\",\n" +
			"      \"status\": \"pending\",\n" +
			"      \"passes\": false,\n" +
			"      \"steps\": null,\n" +
			"      \"dependencies\": null\n" +
			"    },\n" +
			"    {\n" +
			"      \"id\": \"req-the-list-of-primes-in-primes-j\",\n" +
			"      \"category\": \"functional\",\n" +
			"      \"priority\": \"critical\",\n" +
			"      \"description\": \"The list of primes in 'primes.json' contains exactly 1229 prime numbers\",\n" +
			"      \"status\": \"pending\",\n" +
			"      \"passes\": false,\n" +
			"      \"steps\": null,\n" +
			"      \"dependencies\": null\n" +
			"    }\n" +
			"  ]\n" +
			"}\n" +
			"EOF\n" +
			"agent-bridge import feature_list.json\n" +
			"```", nil
	}

	// 2. Planning Phase (TPM)
	if strings.Contains(lowerPrompt, "technical program manager") || strings.Contains(lowerPrompt, "tpm") {
		return `[
  {
    "title": "ID:[PRIMES] Prime Number Script",
    "description": "Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'. The JSON format must have a single key 'primes' containing the list of integers. The script MUST be named 'primes.py'. The output file MUST be named 'primes.json'. Implement prime calculation logic in primes.py, output results to primes.json, validate that the output file contains a 'primes' list, verify that exactly 1229 primes are calculated, and commit primes.json to the repository. Repo: https://github.com/process-failed-successfully/recac-jira-e2e",
    "type": "Task",
    "priority": "High",
    "labels": ["backend", "python"]
  }
]`, nil
	}

	// 3. QA Agent / Manager / Cleaner
	if strings.Contains(lowerPrompt, "qa agent") || strings.Contains(lowerPrompt, "role - qa agent") {
		return "```bash\n" +
			"agent-bridge signal QA_PASSED true\n" +
			"```", nil
	}
	if strings.Contains(lowerPrompt, "manager") || strings.Contains(lowerPrompt, "project manager") {
		return "```bash\n" +
			"agent-bridge signal PROJECT_SIGNED_OFF true --privileged\n" +
			"```", nil
	}

	// 4. Execution Phase (Coding Agent)
	// Triggers for prime-python scenario
	if (strings.Contains(lowerPrompt, "prime") && strings.Contains(lowerPrompt, "python")) ||
		strings.Contains(lowerPrompt, "primes.py") ||
		strings.Contains(lowerPrompt, "id:[primes]") {

		if strings.Contains(lowerPrompt, "generate ticket") {
			// Fallback to TPM if ambiguous
			return `[{"title": "Implement Primes", "description": "Implement primes.py"}]`, nil
		}

		return "```bash\n" +
			"set -ex\n" +
			"# Create the python script\n" +
			"cat << 'EOF' > primes.py\n" +
			"import json\n\n" +
			"def is_prime(n):\n" +
			"    if n <= 1:\n" +
			"        return False\n" +
			"    for i in range(2, int(n**0.5) + 1):\n" +
			"        if n % i == 0:\n" +
			"            return False\n" +
			"    return True\n\n" +
			"primes = [n for n in range(2, 10000) if is_prime(n)]\n\n" +
			"with open('primes.json', 'w') as f:\n" +
			"    json.dump({'primes': primes}, f)\n" +
			"EOF\n\n" +
			"# Execute the script\n" +
			"python3 primes.py\n\n" +
			"# Verify output exists\n" +
			"ls -l primes.json\n\n" +
			"# Import requirements (idempotent)\n" +
			"cat <<EOF > feature_list.json\n" +
			"{\n" +
			"  \"project_name\": \"MFLP-12559\",\n" +
			"  \"features\": [\n" +
			"    {\"id\": \"req-the-script-primes-py-is-implem\", \"status\": \"pending\"},\n" +
			"    {\"id\": \"req-the-output-is-written-to-a-fil\", \"status\": \"pending\"},\n" +
			"    {\"id\": \"req-the-primes-json-file-contains-\", \"status\": \"pending\"},\n" +
			"    {\"id\": \"req-the-list-of-primes-in-primes-j\", \"status\": \"pending\"}\n" +
			"  ]\n" +
			"}\n" +
			"EOF\n" +
			"agent-bridge import feature_list.json\n\n" +
			"# Mark features as done\n" +
			"agent-bridge feature set req-the-script-primes-py-is-implem --status done --passes true\n" +
			"agent-bridge feature set req-the-output-is-written-to-a-fil --status done --passes true\n" +
			"agent-bridge feature set req-the-primes-json-file-contains- --status done --passes true\n" +
			"agent-bridge feature set req-the-list-of-primes-in-primes-j --status done --passes true\n\n" +
			"# Commit changes\n" +
			"git add primes.py primes.json\n" +
			"git commit -m \"Implement primes calculation\" || echo \"nothing to commit\"\n\n" +
			"# Signal completion\n" +
			"agent-bridge signal PROJECT_SIGNED_OFF true --privileged\n" +
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
