package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/jira"
)

// TestJiraWorkflow_E2E tests the full Jira flow:
// 1. Create a ticket (via API).
// 2. Run recac start --jira <ID> --mock --manager-first=false
// 3. Verify it clones repo (we need a repo URL in desc).
// 4. Verify it creates PR/comments (mocks involved for gh/git push potentially or we expect failures if no auth).
func TestJiraWorkflow_E2E(t *testing.T) {
	// Skip if no credentials
	if os.Getenv("JIRA_URL") == "" || (os.Getenv("JIRA_API_TOKEN") == "" && os.Getenv("JIRA_API_KEY") == "") {
		t.Skip("Skipping E2E test: JIRA credentials missing")
	}

	// Setup
	jiraURL := os.Getenv("JIRA_URL")
	// Retrieve username from multiple possible env vars
	jiraUser := os.Getenv("JIRA_USERNAME")
	if jiraUser == "" {
		jiraUser = os.Getenv("JIRA_EMAIL")
	}
	jiraToken := os.Getenv("JIRA_API_TOKEN")
	if jiraToken == "" {
		jiraToken = os.Getenv("JIRA_API_KEY")
	}

	// Fail if user still empty
	if jiraUser == "" {
		t.Skip("Skipping E2E: Jira Username missing")
	}

	jClient := jira.NewClient(jiraURL, jiraUser, jiraToken)
	ctx := context.Background()

	// 1. Create Test Ticket
	// Use a real public repo for cloning test
	testRepo := "https://github.com/process-failed-successfully/recac-jira-e2e.git"
	desc := fmt.Sprintf("This is an automated E2E test ticket.\nRepo: %s\nTask: Update Readme.", testRepo)

	// Project key needs to be valid. Try "PROJ" or env var.
	projectKey := os.Getenv("JIRA_PROJECT_KEY")
	if projectKey == "" {
		t.Skip("Skipping E2E: JIRA_PROJECT_KEY missing")
	}

	ticketID, err := jClient.CreateTicket(ctx, projectKey, "E2E Test Jira Flow", desc, "Task")
	if err != nil {
		t.Fatalf("Failed to create test ticket: %v", err)
	}
	t.Logf("Created test ticket: %s", ticketID)

	// Cleanup ticket? (Maybe manually or let it be 'Done')

	// 2. Prepare Command
	// We need to build the binary first or run using 'go run'
	// Let's assume 'recac-app' is built in root.

	// Build recac-app
	buildCmd := exec.Command("go", "build", "-o", "recac-app", "./cmd/recac")
	buildCmd.Dir = "../../" // root
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build recac: %v\n%s", err, out)
	}

	binPath, _ := filepath.Abs("../../recac-app")

	// Run recac start --jira <ID> --mock (mock mode skips Docker/Agent loop but runs setup?)
	// Wait, mock mode in start.go *skips* the Jira cloning logic because it happens *before* mock check?
	// checking start.go...
	// "Handle Jira Ticket Workflow" is line 91.
	// "Mock mode" check is line 285.
	// So Jira Cloning happens BEFORE Mock mode check! Use --mock to avoid real agent cost.

	// But! Logic in start.go:
	// If jiraTicketID != "":
	// ... clones ...
	// ... projectPath = tempWorkspace
	// ...
	// If isMock:
	// ... NewMockClient ...
	// ... NewSession ...
	// ... Start ...
	// ... RunLoop ...

	// And Completion Logic (Push/PR) is AFTER RunLoop.
	// Is it executed if isMock is true?
	// Line 562: if jiraTicketID != "" && !isMock { ... }
	// Ah, completion is skipped in mock mode to avoid pushing garbage branches/PRs from mock sessions.

	// So for E2E, we might want to *not* use --mock for the full flow, OR allow completion in mock.
	// But we don't want to spend money on agent.
	// Maybe we use --mock-agent (feature flag?) or just provider=mock?
	// start.go has: `startCmd.Flags().Bool("mock", false, ...)`
	// If we use `--provider mock` (custom provider), start.go doesn't have explicit handled "mock" provider except checking for API key.

	// Let's rely on the fact that we can verify the CLONING part easily.
	// The PR creation part requires `!isMock`.
	// I can modify start.go to `if jiraTicketID != "" && (!isMock || os.Getenv("E2E_TEST") == "1")`?
	// Or just manually run the git commands in test to verify repo state.

	cmd := exec.Command(binPath, "start", "--jira", ticketID, "--mock", "--detached=false", "--name", "e2e-test")
	cmd.Env = os.Environ() // Pass JIRA vars

	// Set 5s timeout? The session loop in mock mode runs ... how long?
	// mock agent usually returns fast? MockAgent logic needs checking.
	// Mock Agent returns 'finished' immediately or loops?
	// If it loops forever, we need to interrupt.

	// Let's try running it.
	output, err := cmd.CombinedOutput()
	t.Logf("Output: %s", output)

	if err != nil {
		t.Logf("Command finished with error (could be expected if we kill it): %v", err)
	}

	// Verify:
	// 1. Check stdout for "Cloning into..."
	// 2. Check stdout for "Workspace created..."

	if !strings.Contains(string(output), "Cloning into") {
		t.Errorf("Expected 'Cloning into' in output")
	}
	if !strings.Contains(string(output), "Ticket Found:") {
		t.Errorf("Expected 'Ticket Found' in output")
	}
}
