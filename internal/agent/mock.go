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

	// Heuristic for E2E Tests: [PRIMES] Scenario
	// We also check for the feature ID `req-primes-py-exists` because the Coding Agent prompt
	// might not contain the original [PRIMES] tag from the App Spec/Ticket if it uses the feature description.
	// We also check for the project name "prime-python" or the explicit "primes.py" file to ensure robustness.
	if strings.Contains(prompt, "[PRIMES]") || strings.Contains(prompt, "req-primes-py-exists") ||
		strings.Contains(prompt, "prime-python") || strings.Contains(prompt, "primes.py") {
		// 1. Technical Program Manager (Ticket Generation)
		// Detects "Technical Program Manager" role or ticket generation instructions
		if (strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "CRITICAL INSTRUCTION FOR TICKET GENERATION")) &&
			!strings.Contains(prompt, "YOUR ROLE - CODING AGENT") {
			return `[
  {
    "title": "[PRIMES] Create Prime Number Script",
    "description": "Create a python script named 'primes.py'. It MUST be python.\nIt must calculate all prime numbers less than 10,000 and output to a file named 'primes.json'.\nIMPORTANT: You MUST use a bash block to create the file (e.g., cat << 'EOF' > primes.py). Do not output raw python code.\nCommit 'primes.py' and 'primes.json' IMMEDIATELY. Use 'git add -f primes.json' to ensure it is tracked.\nThe JSON format must have a single key 'primes' containing the list of integers.\nExample: ` + "`" + `{\"primes\": [2, 3, 5, ...]}` + "`" + `.\nIMPORTANT: Ensure the FINAL primes.json committed to the repository contains ALL primes less than 10,000 (Exactly 1229 primes).\nDo not truncate it for testing or reporting - the verification script expects the full list.\nKeep the code absolutely minimal. Finish as quickly as possible.",
    "type": "Task",
    "blocked_by": [],
    "acceptance_criteria": [],
    "children": []
  }
]`, nil
		}

		// 2. Coding Agent (Implementation) - MOVED UP PRIORITY
		// Detects "Coding Agent" role
		// We check this BEFORE Initializer because the coding prompt often contains "feature_list.json" (via cat command)
		// which triggers the Initializer heuristic if checked first.
		// We prioritize "YOUR ROLE - CODING AGENT" as a definitive signal, ignoring exclusions if present.
		// If "Implement the solution" is the trigger, we still check exclusions to avoid false positives from PM/QA.
		isCodingAgent := strings.Contains(prompt, "YOUR ROLE - CODING AGENT")
		isImplInstruction := strings.Contains(prompt, "Implement the solution")
		isNotPMOrQA := !strings.Contains(prompt, "ROLE: Project Manager") && !strings.Contains(prompt, "QA")

		if isCodingAgent || (isImplInstruction && isNotPMOrQA) {
			return "```bash\n" +
				"cat << 'EOF' > primes.py\n" +
				"import json\n\n" +
				"def is_prime(n):\n" +
				"    if n < 2: return False\n" +
				"    for i in range(2, int(n**0.5) + 1):\n" +
				"        if n % i == 0: return False\n" +
				"    return True\n\n" +
				"primes = [i for i in range(10000) if is_prime(i)]\n" +
				"with open('primes.json', 'w') as f:\n" +
				"    json.dump({'primes': primes}, f)\n" +
				"EOF\n\n" +
				"python3 primes.py\n" +
				"git add primes.py primes.json\n" +
				"git commit -m \"Add primes.py and primes.json\"\n" +
				"agent-bridge feature set req-primes-py-exists --status done --passes true\n" +
				"```", nil
		}

		// 3. Initializer (Feature List Generation)
		// Detects "Initializer" role or feature list requests
		// Also exclude Project Manager and QA roles.
		if (strings.Contains(prompt, "Initialize the project") || strings.Contains(prompt, "feature_list.json")) &&
			!strings.Contains(prompt, "ROLE: Project Manager") && !strings.Contains(prompt, "QA") {
			return "```bash\n" +
				"cat <<EOF | agent-bridge import\n" +
				"{\n" +
				"  \"project_name\": \"prime-python\",\n" +
				"  \"features\": [\n" +
				"    {\n" +
				"      \"id\": \"req-primes-py-exists\",\n" +
				"      \"description\": \"Create primes.py script\",\n" +
				"      \"priority\": \"1\"\n" +
				"    }\n" +
				"  ]\n" +
				"}\n" +
				"EOF\n" +
				"```", nil
		}
	}

	// 4. QA Agent (Verification)
	// Triggers if prompt identifies as QA Agent
	if strings.Contains(prompt, "QA AGENT") || strings.Contains(prompt, "verify the project") {
		return "```bash\n" +
			"echo \"Running verification...\"\n" +
			"if [ -f primes.py ] && [ -f primes.json ]; then\n" +
			"  echo \"Files exist.\"\n" +
			"else\n" +
			"  echo \"Files missing.\"\n" +
			"  exit 1\n" +
			"fi\n" +
			"agent-bridge signal QA_PASSED true\n" +
			"```", nil
	}

	// 5. Project Manager (Sign-off)
	// Triggers if prompt identifies as Project Manager.
	// We use strict header matching ("ROLE - PROJECT MANAGER") if available,
	// or "Manager Review" ONLY if "YOUR ROLE - CODING AGENT" is NOT present (to avoid confusing feedback with role).
	isProjectManager := strings.Contains(prompt, "ROLE - PROJECT MANAGER") ||
		(strings.Contains(prompt, "Manager Review") && !strings.Contains(prompt, "YOUR ROLE - CODING AGENT"))

	if isProjectManager {
		return "```bash\n" +
			"echo \"Project approved.\"\n" +
			"agent-bridge signal PROJECT_SIGNED_OFF true\n" +
			"```", nil
	}

	// Return a generic mock response that shows the agent received the prompt
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
