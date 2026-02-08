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
func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.forcedResponse != "" {
		return m.forcedResponse, nil
	}

	// Heuristics for E2E Smoke Tests

	// 1. Ticket Generation
	if strings.Contains(prompt, "critical instruction for ticket generation") || strings.Contains(prompt, "Technical Program Manager") {
		return `[
  {
    "title": "ID:[PRIMES] Prime Number Script Task",
    "description": "Implement a Python script to calculate prime numbers.",
    "type": "Task",
    "labels": ["backend", "python"],
    "acceptance_criteria": [
      "The script 'primes.py' is implemented and calculates all prime numbers less than 10,000",
      "The results are output to a file named 'primes.json' in the correct JSON format",
      "The output file 'primes.json' contains a 'primes' list",
      "Exactly 1229 primes are calculated and included in the 'primes.json' file",
      "The 'primes.json' file is committed to the repository"
    ],
    "dependencies": [],
    "story_points": 3
  }
]`, nil
	}

	// 2. Initializer (Feature Import)
	// We handle [PRIMES] specifically to ensure correct feature list creation
	if strings.Contains(prompt, "feature_list.json") && (strings.Contains(prompt, "INITIALIZER") || strings.Contains(prompt, "Initialize")) {
		if strings.Contains(prompt, "[PRIMES]") {
			return "```bash\ncat <<'EOF' > feature_list.json\n{\n  \"project_name\": \"primes\",\n  \"features\": [\n    {\n      \"id\": \"req-the-script-primes-py-is-implem\",\n      \"category\": \"functional\",\n      \"priority\": \"MVP\",\n      \"description\": \"The script 'primes.py' is implemented and calculates all prime numbers less than 10,000\",\n      \"status\": \"pending\",\n      \"steps\": [\"Run script\"],\n      \"passes\": false,\n      \"dependencies\": {\n        \"depends_on_ids\": [],\n        \"exclusive_write_paths\": [],\n        \"read_only_paths\": []\n      }\n    },\n    {\n      \"id\": \"req-the-results-are-output-to-a-fi\",\n      \"category\": \"functional\",\n      \"priority\": \"MVP\",\n      \"description\": \"The results are output to a file named 'primes.json' in the correct JSON format\",\n      \"status\": \"pending\",\n      \"steps\": [\"Check file exists\"],\n      \"passes\": false,\n      \"dependencies\": {\n        \"depends_on_ids\": [],\n        \"exclusive_write_paths\": [],\n        \"read_only_paths\": []\n      }\n    },\n    {\n      \"id\": \"req-the-output-file-primes-json-co\",\n      \"category\": \"functional\",\n      \"priority\": \"MVP\",\n      \"description\": \"The output file 'primes.json' contains a 'primes' list\",\n      \"status\": \"pending\",\n      \"steps\": [\"Check JSON content\"],\n      \"passes\": false,\n      \"dependencies\": {\n        \"depends_on_ids\": [],\n        \"exclusive_write_paths\": [],\n        \"read_only_paths\": []\n      }\n    },\n    {\n      \"id\": \"req-exactly-1229-primes-are-calcul\",\n      \"category\": \"functional\",\n      \"priority\": \"MVP\",\n      \"description\": \"Exactly 1229 primes are calculated and included in the 'primes.json' file\",\n      \"status\": \"pending\",\n      \"steps\": [\"Count primes\"],\n      \"passes\": false,\n      \"dependencies\": {\n        \"depends_on_ids\": [],\n        \"exclusive_write_paths\": [],\n        \"read_only_paths\": []\n      }\n    },\n    {\n      \"id\": \"req-the-primes-json-file-is-commit\",\n      \"category\": \"functional\",\n      \"priority\": \"MVP\",\n      \"description\": \"The 'primes.json' file is committed to the repository\",\n      \"status\": \"pending\",\n      \"steps\": [\"Check git log\"],\n      \"passes\": false,\n      \"dependencies\": {\n        \"depends_on_ids\": [],\n        \"exclusive_write_paths\": [],\n        \"read_only_paths\": []\n      }\n    }\n  ]\n}\nEOF\n```", nil
		}
		// Generic Initializer fallback (valid JSON structure)
		return "```bash\necho '{\"project_name\": \"unknown\", \"features\": []}' > feature_list.json\n```", nil
	}

	// 3. QA Agent
	if strings.Contains(prompt, "YOUR ROLE - QA AGENT") {
		return "```bash\nagent-bridge signal QA_PASSED true\n```", nil
	}

	// 4. Project Manager
	if strings.Contains(prompt, "PROJECT MANAGER") {
		return "```bash\nagent-bridge signal PROJECT_SIGNED_OFF true\n```", nil
	}

	// 5. Implementation (Primes)
	// We check for various triggers including the specific feature ID found in logs
	// Note: We deliberately exclude prompts that contain "YOUR ROLE - QA AGENT" or "PROJECT MANAGER"
	// effectively by placing this check AFTER those specific roles.
	if strings.Contains(prompt, "calculate primes") ||
		strings.Contains(prompt, "[PRIMES]") ||
		strings.Contains(prompt, "primes.py") ||
		strings.Contains(prompt, "Prime Number Script") ||
		strings.Contains(prompt, "req-the-script-primes-py-is-implem") ||
		strings.Contains(prompt, "YOUR ROLE - CODING AGENT") {
		return "I will implement the prime number calculation script.\n\n" +
			"```bash\n" +
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
			"primes = [x for x in range(10000) if is_prime(x)]\n" +
			"\n" +
			"with open('primes.json', 'w') as f:\n" +
			"    json.dump({\"primes\": primes}, f)\n" +
			"EOF\n" +
			"\n" +
			"python3 primes.py\n" +
			"\n" +
			"git config user.email \"agent@recac.io\"\n" +
			"git config user.name \"RECAC Agent\"\n" +
			"git add primes.py primes.json\n" +
			"git commit -m \"Add primes script and results\"\n" +
			"git push || echo \"Push skipped\"\n" +
			"\n" +
			"agent-bridge feature set req-the-script-primes-py-is-implem --status done --passes true\n" +
			"agent-bridge feature set req-the-results-are-output-to-a-fi --status done --passes true\n" +
			"agent-bridge feature set req-the-output-file-primes-json-co --status done --passes true\n" +
			"agent-bridge feature set req-exactly-1229-primes-are-calcul --status done --passes true\n" +
			"agent-bridge feature set req-the-primes-json-file-is-commit --status done --passes true\n" +
			"```\n" +
			"\n" +
			"COMPLETED\n", nil
	}

	// Default Echo
	// We include a harmless bash command to prevent "NO-OP LOOP" circuit breaker in Runner
	safePromptPreview := strings.ReplaceAll(truncateString(prompt, 100), "`", "")
	response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request.\n\nPrompt preview: %s...\n\n```bash\necho \"Mock Agent: Processing request (Fallback)\"\n```",
		m.responsePrefix, len(prompt), safePromptPreview)
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
