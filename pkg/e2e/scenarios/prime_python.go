package scenarios

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
)

type PrimePythonScenario struct{}

func (s *PrimePythonScenario) Name() string {
	return "prime-python"
}

func (s *PrimePythonScenario) Description() string {
	return "A simple test asking for a Python script that outputs primes < 10000 in JSON."
}

func (s *PrimePythonScenario) Generate(uniqueID string, repoURL string) []TicketSpec {
	return []TicketSpec{
		{
			ID:      "PRIMES",
			Summary: fmt.Sprintf("[%s] Create Prime Number Script", uniqueID),
			Desc: fmt.Sprintf(`Create a python script named 'primes.py'.
It must calculate all prime numbers less than 10,000.
It must print the result to stdout as a JSON object with a single key 'primes' containing the list of integers.
Example: %s{"primes": [2, 3, 5, ...]}%s.
Do not output anything else to stdout.
Keep the code absolutely minimal. Finish as quickly as possible.

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
		// Fallback to any agent branch if specific fail
		log.Printf("Warning: Specific branch for %s not found, checking generic...", ticketKey)
		branch, err = getAgentBranch(repoPath)
		if err != nil {
			return err
		}
	}
	fmt.Printf("Verifying branch: %s\n", branch)

	// Checkout branch
	checkoutCmd := exec.Command("git", "checkout", branch)
	checkoutCmd.Dir = repoPath
	if out, err := checkoutCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to checkout %s: %v\nOutput: %s", branch, err, out)
	}

	// Check file existence
	scriptPath := "primes.py" // Relative to repo root

	// Run script
	cmd := exec.Command("python3", scriptPath)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Try python just in case
		cmd = exec.Command("python", scriptPath)
		cmd.Dir = repoPath
		out, err = cmd.CombinedOutput()
		if err != nil {
			// List files to help debugging
			lsCmd := exec.Command("ls", "-R")
			lsCmd.Dir = repoPath
			lsOut, _ := lsCmd.CombinedOutput()
			return fmt.Errorf("failed to run primes.py: %v\nOutput:\n%s\nFiles in repo:\n%s", err, string(out), string(lsOut))
		}
	}

	// Parse JSON
	var result struct {
		Primes []int `json:"primes"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return fmt.Errorf("failed to parse JSON output: %v\nOutput was: %s", err, string(out))
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

	fmt.Printf("Verification Successful: Found %d primes correctly.\n", len(primes))
	return nil
}

func init() {
	Register(&PrimePythonScenario{})
}
