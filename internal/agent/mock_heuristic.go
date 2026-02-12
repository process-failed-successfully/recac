package agent

import (
	"strings"
)

// getMockResponse returns a tailored response based on the prompt content
func getMockResponse(prompt string) string {
	promptLower := strings.ToLower(prompt)

	// 1. TPM Agent (Scenario Generator)
	// Expects: JSON array of tickets
	if strings.Contains(prompt, "Technical Program Manager") || strings.Contains(prompt, "TPM") {
		return `[
  {
    "summary": "Implement Prime Number Generator",
    "description": "Create a python script that calculates prime numbers up to a limit.",
    "type": "Epic",
    "id": "PRIMES",
    "children": [
      {
        "summary": "Create primes.py script",
        "description": "Write a Python script 'primes.py' that outputs primes to 'primes.json'.",
        "type": "Task"
      }
    ]
  }
]`
	}

	// 2. Initializer Agent
	// Expects: Bash script to import feature list
	if strings.Contains(prompt, "Initializer Agent") || strings.Contains(prompt, "INITIALIZER AGENT") {
		return "Here is the initialization script:\n\n```bash\ncat << 'EOF' > feature_list.json\n[\n  {\n    \"name\": \"primes-script\",\n    \"description\": \"Generate primes.json using python\",\n    \"files\": [\"primes.py\", \"primes.json\"]\n  }\n]\nEOF\n\n# Import the features using the project ID from environment if available, or default to current context\nPROJECT_ID=\"${RECAC_PROJECT_ID:-$PROJECT_ID}\"\nagent-bridge import --file feature_list.json --project \"$PROJECT_ID\"\n```"
	}

	// 3. Coding Agent (Prime Number Scenario)
	// Expects: Bash script to implement the feature
	if strings.Contains(promptLower, "prime") && (strings.Contains(promptLower, "python") || strings.Contains(promptLower, "script")) {
		return "I will create the prime number generator script.\n\n```bash\n# Create the python script\ncat << 'EOF' > primes.py\nimport json\n\ndef is_prime(n):\n    if n < 2:\n        return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0:\n            return False\n    return True\n\nprimes = [x for x in range(2, 100) if is_prime(x)]\n\nwith open('primes.json', 'w') as f:\n    json.dump(primes, f)\n\nprint(f\"Generated {len(primes)} primes\")\nEOF\n\n# Run it to generate the output file\npython3 primes.py\n\n# Mark feature as completed\nFEATURE_ID=\"primes-script\"\nagent-bridge feature set \"$FEATURE_ID\" --status completed --passes true\n```"
	}

	// 4. QA Agent / Manager
	// Expects: Approval signal
	if strings.Contains(prompt, "QA") || strings.Contains(prompt, "REVIEW") || strings.Contains(prompt, "VERIFY") {
		return "The changes look correct. The primes.json file is generated and valid.\nLGTM.\n\n```bash\nagent-bridge signal set --type QA_PASSED\n```"
	}

	// Default fallback
	return ""
}
