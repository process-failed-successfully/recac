package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"recac/internal/jira"

	"github.com/joho/godotenv"
)

// TestJiraEpicSmoketest_GoCalculator performs a full E2E smoketest:
// 1. Creates an Epic "Build Go Calculator".
// 2. Creates 5 child tickets (Init, Add/Sub, Mul/Div, Main, Tests).
// 3. Runs 'recac start' for each ticket using OpenRouter/Mistral and --auto-merge.
// 4. Verifies that the Epic branch contains all changes.
func TestJiraEpicSmoketest_GoCalculator(t *testing.T) {
	// Try loading .env from root or CWD
	_ = godotenv.Load()
	_ = godotenv.Load("../../.env")

	// 1. Setup & Credentials Check
	jiraURL := os.Getenv("JIRA_URL")
	jiraUser := os.Getenv("JIRA_USERNAME")
	if jiraUser == "" {
		jiraUser = os.Getenv("JIRA_EMAIL")
	}
	jiraToken := os.Getenv("JIRA_API_TOKEN")
	if jiraToken == "" {
		jiraToken = os.Getenv("JIRA_API_KEY")
	}
	projectKey := os.Getenv("JIRA_PROJECT_KEY")

	// OpenRouter check
	openRouterKey := os.Getenv("OPENROUTER_API_KEY")

	if jiraURL == "" || jiraUser == "" || jiraToken == "" || projectKey == "" {
		t.Skip("Skipping E2E: Missing JIRA credentials")
	}
	if openRouterKey == "" {
		t.Skip("Skipping E2E: Missing OPENROUTER_API_KEY")
	}

	ctx := context.Background()
	jClient := jira.NewClient(jiraURL, jiraUser, jiraToken)

	// Build recac binary
	root, _ := filepath.Abs("../../") // Assuming running from tests/e2e
	binPath := filepath.Join(root, "recac-app-smoketest")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/recac")
	buildCmd.Dir = root
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build recac: %v\n%s", err, out)
	}
	defer os.Remove(binPath) // Cleanup binary

	// 2. Create Epic
	timestamp := time.Now().Format("20060102-150405")
	epicSummary := fmt.Sprintf("Go Calc Epic %s", timestamp)
	epicDesc := "Epic to build a simple calculator in Go."
	epicID, err := jClient.CreateTicket(ctx, projectKey, epicSummary, epicDesc, "Epic")
	if err != nil {
		t.Fatalf("Failed to create Epic: %v", err)
	}
	t.Logf("Created Epic: %s", epicID)

	repo := "https://github.com/process-failed-successfully/recac-jira-e2e.git"

	// 2.a Create Epic Branch remotely (Prerequisite for AutoMerge upstream check)
	gitAuthKey := os.Getenv("GITHUB_API_KEY")
	if gitAuthKey == "" {
		t.Fatal("GITHUB_API_KEY not set")
	}
	gitAuthKey = strings.Trim(gitAuthKey, "\"") // Strip quotes if present
	os.Setenv("GITHUB_API_KEY", gitAuthKey)     // Update env for subprocesses

	// Clone to temp
	setupDir := filepath.Join(os.TempDir(), fmt.Sprintf("setup-%s", timestamp))
	repoAuthURL := strings.Replace(repo, "https://github.com/", fmt.Sprintf("https://%s@github.com/", gitAuthKey), 1)

	setupCmd := exec.Command("git", "clone", repoAuthURL, setupDir)
	if out, err := setupCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to clone for setup: %v\n%s", err, out)
	}

	// Create and push epic branch
	epicBranch := "agent-epic/" + epicID
	createBranchCmd := exec.Command("git", "checkout", "-b", epicBranch)
	createBranchCmd.Dir = setupDir
	if out, err := createBranchCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to create epic branch: %v\n%s", err, out)
	}

	pushBranchCmd := exec.Command("git", "push", "origin", epicBranch)
	pushBranchCmd.Dir = setupDir
	if out, err := pushBranchCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to push epic branch: %v\n%s", err, out)
	}
	t.Logf("Created and pushed branch: %s", epicBranch)
	os.RemoveAll(setupDir)

	// 3. Create Child Tickets
	tickets := []struct {
		Summary string
		Desc    string
	}{
		{
			"Init Go Module",
			fmt.Sprintf("Initialize a go module 'calc'.\nRepo: %s", repo),
		},
		{
			"Implement Add and Sub",
			fmt.Sprintf("Create a calculator package with Add(a, b int) int and Sub(a, b int) int functions.\nRepo: %s", repo),
		},
		{
			"Implement Mul and Div",
			fmt.Sprintf("Add Mul(a, b int) int and Div(a, b int) (int, error) functions. Handle division by zero.\nRepo: %s", repo),
		},
		{
			"Implement CLI Main",
			fmt.Sprintf("Create a main.go that uses the calculator package to perform 2+2 and print the result.\nRepo: %s", repo),
		},
		{
			"Add Unit Tests",
			fmt.Sprintf("Add unit tests for all calculator functions.\nRepo: %s", repo),
		},
	}

	ticketIDs := []string{}
	// label := fmt.Sprintf("smoketest-%s", timestamp) // Unused without AddLabel

	for _, ticket := range tickets {
		// Append label to description or fields?
		// CreateChildTicket doesn't support labels arg directly in current client wrapper,
		// but we can assume 'recac start' finds it by ID.
		// However, the test requirement says "make a 5 ticket epic... check all tickets end up in Done".
		// We'll iterate by ID, so label is less critical for finding them, but good for cleanup.

		// Note: CreateChildTicket signature is (ctx, project, summary, description, type, parentID)
		childID, err := jClient.CreateChildTicket(ctx, projectKey, ticket.Summary, ticket.Desc, "Task", epicID)
		if err != nil {
			t.Fatalf("Failed to create child ticket '%s': %v", ticket.Summary, err)
		}
		t.Logf("Created Ticket: %s", childID)
		ticketIDs = append(ticketIDs, childID)

		// Add Label (Best Effort) -- Client doesn't support AddLabel yet
		// _ = jClient.AddLabel(ctx, childID, label)
	}

	// 4. Run recac start for each ticket
	provider := "openrouter"
	model := "mistralai/devstral-2512:free"

	for _, ticketID := range ticketIDs {
		t.Logf("Starting work on ticket: %s", ticketID)

		// Run recac start
		// --auto-merge enabled
		// --allow-dirty to avoid git strictness in test env if needed (though clean is better)
		// --detached=false to block until done
		// --manager-frequency=100 (reduce manager overhead for speed)
		cmd := exec.Command(binPath, "start",
			"--jira", ticketID,
			"--provider", provider,
			"--model", model,
			"--auto-merge",
			"--mock=false",
			"--detached=false",
			"--name", fmt.Sprintf("session-%s", ticketID),
			"--allow-dirty",
		)

		// Pass environment
		cmd.Env = os.Environ()

		// Stream output for debugging hangs
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()
		if err := cmd.Start(); err != nil {
			t.Errorf("Failed to start agent on ticket %s: %v", ticketID, err)
			continue
		}

		// Read output in goroutine to prevent blocking
		done := make(chan bool)
		var outputBuilder strings.Builder

		go func() {
			defer close(done)
			// Simple buffer read
			buf := make([]byte, 1024)
			for {
				n, err := stdout.Read(buf)
				if n > 0 {
					chunk := string(buf[:n])
					fmt.Print(chunk) // Print to test stdout for immediate visibility
					outputBuilder.WriteString(chunk)
				}
				if err != nil {
					break
				}
			}
		}()

		// Also read stderr
		go func() {
			buf := make([]byte, 1024)
			for {
				n, err := stderr.Read(buf)
				if n > 0 {
					chunk := string(buf[:n])
					fmt.Print(chunk)
					outputBuilder.WriteString(chunk)
				}
				if err != nil {
					break
				}
			}
		}()

		if err := cmd.Wait(); err != nil {
			t.Errorf("Agent failed on ticket %s: %v\nOutput:\n%s", ticketID, err, outputBuilder.String())
			continue
		}
		<-done // Wait for stdout to close
		output := outputBuilder.String()

		// Verify success messages
		if !strings.Contains(output, "Project signed off") {
			t.Errorf("Ticket %s did not complete with 'Project signed off'", ticketID)
		}
		if !strings.Contains(output, "Successfully auto-merged") {
			t.Errorf("Ticket %s did not auto-merge", ticketID)
		}
	}

	// 5. Verify Final State
	verifyDir := filepath.Join(os.TempDir(), fmt.Sprintf("verify-%s", timestamp))
	t.Logf("Verifying code in %s", verifyDir)

	gitCmd := exec.Command("git", "clone", "--branch", "agent-epic/"+epicID, repo, verifyDir)
	githubKey := os.Getenv("GITHUB_API_KEY")
	if githubKey != "" {
		repoURLWithAuth := strings.Replace(repo, "https://github.com/", fmt.Sprintf("https://%s@github.com/", githubKey), 1)
		gitCmd = exec.Command("git", "clone", "--branch", "agent-epic/"+epicID, repoURLWithAuth, verifyDir)
	}

	if out, err := gitCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to clone epic branch to verify: %v\n%s", err, out)
	}

	// Assert Files
	expectedFiles := []string{
		"go.mod",
		"calc/calc.go", // Assuming package calc
		"calc/calc_test.go",
		"main.go",
	}

	for _, file := range expectedFiles {
		fullPath := filepath.Join(verifyDir, file)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("Expected file %s not found in epic branch", file)
		}
	}

	// Check for 'main' function
	// Check for 'Add' function
}
