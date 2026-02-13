package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepro_ExtractRequiredFeatures(t *testing.T) {
	// Sample text from prime_python.go
	spec := `### ID:[PRIMES] Prime Number Script

CRITICAL INSTRUCTION: You MUST create exactly ONE ticket. Type: Task.
Do NOT create an Epic. Do NOT create subtasks.
The ID [PRIMES] must map to this single Task.

Implement a python script named 'primes.py' that calculates all prime numbers less than 10,000 and outputs them to a file named 'primes.json'.

The JSON format must have a single key 'primes' containing the list of integers.
Example: {"primes": [2, 3, 5, ...]}

The script MUST be named 'primes.py'.
The output file MUST be named 'primes.json'.

IMPORTANT: You MUST use a bash block to create the file.
Commit 'primes.json' IMMEDIATELY after creating/running the script. Do NOT leave it untracked.

REQUIRED FEATURES:
- Implement prime calculation logic in primes.py
- Output results to primes.json
- Validate that the output file contains a 'primes' list
- Verify that exactly 1229 primes are calculated
- Commit primes.json to the repository

CRITICAL INSTRUCTION FOR TICKET GENERATION:
Create a SINGLE Ticket (Task) for this work. Do not create an Epic or subtasks. The ID [PRIMES] must map to this single Task.
CRITICAL: Do NOT run 'pytest' or any test framework. Do NOT try to create test files. Just run the script and verify 'primes.json' exists.

Repo: https://github.com/process-failed-successfully/recac-jira-e2e`

	features := extractRequiredFeatures(spec)

	assert.NotEmpty(t, features, "Should extract features")

	for i, f := range features {
		t.Logf("Feature %d: ID='%s' Desc='%s'", i, f.ID, f.Description)
		assert.NotEmpty(t, f.ID, "Feature ID should not be empty")
		assert.NotEmpty(t, f.Description, "Feature Description should not be empty")
	}
}
