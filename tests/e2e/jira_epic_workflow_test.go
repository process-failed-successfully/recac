//go:build e2e

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
)

// TestJiraEpicWorkflow_E2E validates the Epic branching strategy:
// 1. Create an Epic ticket.
// 2. Create a child Task ticket linked to the Epic.
// 3. Run recac start --jira <CHILD_ID> --mock.
// 4. Verify that the agent detects the Epic and sets the base branch to epic/<EPIC_ID>.
func TestJiraEpicWorkflow_E2E(t *testing.T) {
	// Skip if no credentials
	if os.Getenv("JIRA_URL") == "" || (os.Getenv("JIRA_API_TOKEN") == "" && os.Getenv("JIRA_API_KEY") == "") {
		t.Skip("Skipping E2E test: JIRA credentials missing")
	}

	// Setup Client
	jiraURL := os.Getenv("JIRA_URL")
	jiraUser := os.Getenv("JIRA_USERNAME")
	if jiraUser == "" {
		jiraUser = os.Getenv("JIRA_EMAIL")
	}
	jiraToken := os.Getenv("JIRA_API_TOKEN")
	if jiraToken == "" {
		jiraToken = os.Getenv("JIRA_API_KEY")
	}

	if jiraUser == "" {
		t.Skip("Skipping E2E: Jira Username missing")
	}

	jClient := jira.NewClient(jiraURL, jiraUser, jiraToken)
	ctx := context.Background()

	// Project Key
	projectKey := os.Getenv("JIRA_PROJECT_KEY")
	if projectKey == "" {
		t.Skip("Skipping E2E: JIRA_PROJECT_KEY missing")
	}

	// 1. Create Epic Ticket
	timestamp := time.Now().Format("20060102150405")
	epicSummary := fmt.Sprintf("E2E Test Epic %s", timestamp)
	epicDesc := "This is an automated E2E test Epic."
	// Note: 'Epic' issue type is standard, but some projects might differ.
	// If this fails, we might need configuration or a fallback (e.g. searching for issue types).
	epicID, err := jClient.CreateTicket(ctx, projectKey, epicSummary, epicDesc, "Epic")
	if err != nil {
		t.Fatalf("Failed to create Epic ticket: %v. Ensure 'Epic' issue type exists in project %s.", err, projectKey)
	}
	t.Logf("Created Epic ticket: %s", epicID)

	// 2. Create Child Task Linked to Epic
	childSummary := fmt.Sprintf("E2E Test Child of %s", epicID)
	// Use the provided repo for creating branches
	testRepo := "https://github.com/process-failed-successfully/recac-jira-e2e.git"
	childDesc := fmt.Sprintf("Child task for E2E.\nRepo: %s", testRepo)

	// Use CreateChildTicket (assumes we added it in previous step)
	childID, err := jClient.CreateChildTicket(ctx, projectKey, childSummary, childDesc, "Task", epicID)
	if err != nil {
		t.Fatalf("Failed to create child ticket: %v", err)
	}
	t.Logf("Created Child Task: %s (Parent: %s)", childID, epicID)

	// 3. Build recac-app
	buildCmd := exec.Command("go", "build", "-o", "recac-app-epic", "./cmd/recac")
	buildCmd.Dir = "../../" // root
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build recac: %v\n%s", err, out)
	}
	binPath, _ := filepath.Abs("../../recac-app-epic")

	// 4. Run recac start
	// We use --mock to avoid real agent costs, but start.go logic for Jira happens *before* mock loop.
	cmd := exec.Command(binPath, "start", "--jira", childID, "--mock", "--detached=false", "--name", "e2e-epic-test")
	cmd.Env = os.Environ() // Pass JIRA vars

	outputBytes, err := cmd.CombinedOutput()
	output := string(outputBytes)
	t.Logf("CLI Output:\n%s", output)

	// 5. Verify Output
	// a. Detected Epic
	expectedEpicMsg := fmt.Sprintf("Detected parent Epic: %s", epicID)
	if !strings.Contains(output, expectedEpicMsg) {
		t.Errorf("Expected output to contain '%s', but it didn't.", expectedEpicMsg)
	}

	// b. Base Branch Set
	expectedBaseBranch := fmt.Sprintf("Epic flow detected. Base branch set to: epic/%s", epicID)
	if !strings.Contains(output, expectedBaseBranch) {
		t.Errorf("Expected output to contain '%s', but it didn't.", expectedBaseBranch)
	}

	// c. Cloning
	if !strings.Contains(output, "Cloning repository into") {
		t.Errorf("Expected cloning message in output.")
	}

	// d. Branch Creation (Ticket Branch)
	// We expect "Creating and switching to branch: agent/<CHILD_ID>..."
	// The timestamp part is dynamic, so check prefix
	expectedBranchPrefix := fmt.Sprintf("Creating and switching to branch: agent/%s", childID)
	if !strings.Contains(output, expectedBranchPrefix) {
		t.Errorf("Expected output to contain branch creation '%s...'", expectedBranchPrefix)
	}

	// Cleanup? (Optional, maybe delete tickets if possible)
}
