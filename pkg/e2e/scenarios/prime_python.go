package scenarios

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type PrimePythonScenario struct{}

func (s *PrimePythonScenario) Name() string {
	return "prime-python"
}

func (s *PrimePythonScenario) Description() string {
	return "A simple test asking for a Python script that outputs primes < 10000 in JSON."
}

func (s *PrimePythonScenario) AppSpec(repoURL string) string {
	return fmt.Sprintf(`### ID:[PRIMES] Prime Number Script

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

RESTRICTIONS:
- Do NOT create test files (e.g., test_primes.py).
- Do NOT use pytest or unittest.
- JUST run the script to verify output.

EXECUTION STEPS:
1. Create 'primes.py' (must output to 'primes.json').
2. RUN the script: 'python3 primes.py'.
3. VERIFY the output count: python3 -c "import json; print(f'Primes count: {len(json.load(open('primes.json'))['primes'])}')"
   (Should be 1229)
4. Add BOTH files to git: 'git add primes.py primes.json'.
5. Commit: 'git commit -m "Add primes script and output"'

Repo: %s`, repoURL)
}

func (s *PrimePythonScenario) Generate(uniqueID string, repoURL string) []TicketSpec {
	return []TicketSpec{
		{
			ID:      "PRIMES",
			Summary: fmt.Sprintf("[%s] Create Prime Number Script", uniqueID),
			Desc: fmt.Sprintf(`Create a python script named 'primes.py' that calculates primes < 10000 and writes them to 'primes.json'.

RESTRICTIONS:
- Do NOT create unit tests.
- Do NOT truncate the list.

EXECUTION STEPS (FOLLOW EXACTLY):
1. Create 'primes.py' using cat.
2. RUN 'python3 primes.py' immediately.
3. VERIFY the result: python3 -c "import json; print(f'Primes count: {len(json.load(open('primes.json'))['primes'])}')"
   (Must output 1229)
4. 'git add -f primes.py primes.json'
5. 'git commit -m "Add primes"'

Format: %s{"primes": [2, 3, ...]}%s.

Repo: %s`, "`", "`", repoURL),
			Type: "Task",
		},
	}
}

func (s *PrimePythonScenario) Verify(repoPath string, ticketKeys map[string]string) error {
	ticketKey, ok := ticketKeys["PRIMES"]
	if !ok {
		return fmt.Errorf("PRIMES ticket key not found")
	}

	// Helper to find specific agent branch
	branch, err := getSpecificAgentBranch(repoPath, ticketKey)
	if err != nil {
		return fmt.Errorf("specific branch for %s not found: %w", ticketKey, err)
	}
	fmt.Printf("Verifying branch: %s\n", branch)

	// Checkout branch
	checkoutCmd := exec.Command("git", "checkout", branch)
	checkoutCmd.Dir = repoPath
	if out, err := checkoutCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to checkout %s: %v\nOutput: %s", branch, err, out)
	}

	// Check file existence
	jsonPath := "primes.json"
	scriptPath := "primes.py"

	var out []byte

	// 1. Try reading primes.json first (Deterministic Output)
	fullJsonPath := filepath.Join(repoPath, jsonPath)
	var shouldUseFile bool
	if _, err := os.Stat(fullJsonPath); err == nil {
		fmt.Printf("Found %s, checking content...\n", jsonPath)
		fileOut, err := os.ReadFile(fullJsonPath)
		if err == nil && len(fileOut) > 0 {
			// Check if it's a valid non-empty JSON
			var tempResult struct {
				Primes []int `json:"primes"`
			}
			// We only use the file if it parses correctly AND has data
			if json.Unmarshal(fileOut, &tempResult) == nil && len(tempResult.Primes) > 0 {
				out = fileOut
				shouldUseFile = true
				fmt.Printf("Valid content found in %s, verifying...\n", jsonPath)
			} else {
				fmt.Printf("%s exists but is empty or invalid, falling back to execution...\n", jsonPath)
			}
		}
	}

	if !shouldUseFile {
		// 2. Fallback to running the script
		fmt.Printf("Generating output using %s\n", scriptPath)
		cmd := exec.Command("python3", scriptPath)
		cmd.Dir = repoPath
		cmdOut, err := cmd.CombinedOutput()
		if err != nil {
			// Try python just in case
			cmd = exec.Command("python", scriptPath)
			cmd.Dir = repoPath
			cmdOut, err = cmd.CombinedOutput()
			if err != nil {
				// List files to help debugging
				lsCmd := exec.Command("ls", "-R")
				lsCmd.Dir = repoPath
				lsOut, _ := lsCmd.CombinedOutput()
				return fmt.Errorf("failed to run %s: %v\nOutput:\n%s\nFiles in repo:\n%s", scriptPath, err, string(cmdOut), string(lsOut))
			}
		}
		// After running script, try reading the file it produced
		fileOut, err := os.ReadFile(fullJsonPath)
		if err != nil {
			return fmt.Errorf("script ran but failed to read %s: %v\nScript Output: %s", jsonPath, err, string(cmdOut))
		}
		out = fileOut
		fmt.Printf("Successfully generated and read %s\n", jsonPath)
	}

	// Parse JSON
	var result struct {
		Primes []int `json:"primes"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return fmt.Errorf("failed to parse JSON from %s: %v\nContent was: %s", jsonPath, err, string(out))
	}

	// Verify Primes
	primes := result.Primes
	if len(primes) == 0 {
		return fmt.Errorf("primes list is empty")
	}

	// Basic checks
	if primes[0] != 2 {
		return fmt.Errorf("first prime is not 2, got %d", primes[0])
	}
	if primes[len(primes)-1] >= 10000 {
		return fmt.Errorf("found prime >= 10000: %d", primes[len(primes)-1])
	}

	// Count check (There are 1229 primes < 10000)
	expectedCount := 1229
	if len(primes) != expectedCount {
		return fmt.Errorf("expected %d primes, got %d", expectedCount, len(primes))
	}

	// Last prime check (Largest prime < 10000 is 9973)
	expectedLast := 9973
	lastPrime := int(primes[len(primes)-1])
	if lastPrime != expectedLast {
		return fmt.Errorf("expected last prime to be %d, got %d", expectedLast, lastPrime)
	}

	fmt.Printf("Verification Successful: Found %d primes correctly.\n", len(primes))
	return nil
}

func init() {
	Register(&PrimePythonScenario{})
}
