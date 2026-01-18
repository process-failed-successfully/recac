//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"recac/internal/docker"
	"recac/internal/jira"
	"recac/internal/orchestrator"
	"recac/internal/runner"

	"github.com/joho/godotenv"
)

func TestOrchestrator_Poller_E2E(t *testing.T) {
	// 1. Load Credentials
	_ = godotenv.Load("../../.env")

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

	if jiraURL == "" || jiraUser == "" || jiraToken == "" || projectKey == "" {
		t.Skip("Skipping E2E: Missing JIRA credentials")
	}

	ctx := context.Background()
	client := jira.NewClient(jiraURL, jiraUser, jiraToken)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// 2. Create Test Data
	timestamp := time.Now().Format("20060102150405")
	summary := fmt.Sprintf("Orchestrator E2E Test %s", timestamp)
	desc := fmt.Sprintf("Test Ticket for Orchestrator E2E.\nRepo: https://github.com/process-failed-successfully/recac-e2e-repo")

	key, err := client.CreateTicket(ctx, projectKey, summary, desc, "Task", nil)
	if err != nil {
		t.Fatalf("Failed to create ticket: %v", err)
	}
	t.Logf("Created test ticket: %s", key)

	defer func() {
		if err := client.DeleteIssue(ctx, key); err != nil {
			t.Logf("Warning: Failed to cleanup ticket %s: %v", key, err)
		} else {
			t.Logf("Deleted clean up ticket: %s", key)
		}
	}()

	targetJQL := fmt.Sprintf("key = %s", key)

	// 3. Setup Poller
	poller := orchestrator.NewJiraPoller(client, targetJQL)

	// 4. Poll
	t.Log("Polling for work...")
	items, err := poller.Poll(ctx, logger)
	if err != nil {
		t.Fatalf("Poll failed: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("Expected 1 work item, got %d", len(items))
	}

	item := items[0]
	if item.ID != key {
		t.Errorf("Expected item ID %s, got %s", key, item.ID)
	}

	// 5. Claim - Removed as Claim is no longer in Poller interface
	// Instead, we can simulate what would happen, or just verify status update via UpdateStatus if needed.
	// But simply polling successfully is good enough for this unit test of Poller (except it's E2E).
	// Let's just update status to mimic claim
	t.Log("Claiming work (simulated)...")
	if err := poller.UpdateStatus(ctx, item, "In Progress", "Claimed by E2E test"); err != nil {
		t.Fatalf("Claim (UpdateStatus) failed: %v", err)
	}

	// 6. Verify Status Change (In Progress)
	ticket, err := client.GetTicket(ctx, key)
	if err != nil {
		t.Fatalf("Failed to fetch ticket for verification: %v", err)
	}

	fields := ticket["fields"].(map[string]interface{})
	statusMap := fields["status"].(map[string]interface{})
	statusName := statusMap["name"].(string)

	t.Logf("Ticket Status: %s", statusName)

	if statusName != "In Progress" {
		t.Errorf("Expected status 'In Progress', got '%s'", statusName)
	}
}

func TestOrchestrator_FullFlow_E2E(t *testing.T) {
	// 1. Load Credentials
	_ = godotenv.Load("../../.env")

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

	if jiraURL == "" || jiraUser == "" || jiraToken == "" || projectKey == "" {
		t.Skip("Skipping E2E: Missing JIRA credentials")
	}

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// 2. Build Docker Image (recac-agent:e2e)
	t.Log("Building Docker image for E2E...")
	rootDir, _ := filepath.Abs("../../")
	cmd := exec.Command("docker", "build", "-t", "recac-agent:e2e", "-f", "Dockerfile", ".")
	cmd.Dir = rootDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build docker image: %v\n%s", err, out)
	}

	// 3. Create Jira Ticket
	// Ensure GITHUB_API_KEY is present for the agent to push code
	if os.Getenv("GITHUB_API_KEY") == "" {
		t.Fatal("Start E2E Failed: GITHUB_API_KEY not set in environment (required for agent push)")
	}

	jClient := jira.NewClient(jiraURL, jiraUser, jiraToken)
	timestamp := time.Now().Format("20060102150405")
	summary := fmt.Sprintf("Orchestator Agent Test %s", timestamp)
	repoURL := "https://github.com/process-failed-successfully/recac-jira-e2e.git"
	desc := fmt.Sprintf("Create a file called 'agent_%s.txt' with content 'Hello from Orchestrator'.\nRepo: %s", timestamp, repoURL)

	key, err := jClient.CreateTicket(ctx, projectKey, summary, desc, "Task", nil)
	if err != nil {
		t.Fatalf("Failed to create ticket: %v", err)
	}
	t.Logf("Created ticket: %s", key)

	defer func() {
		// Cleanup ticket
		_ = jClient.DeleteIssue(ctx, key)
	}()

	// 4. Setup Orchestrator Components
	// Filter to avoid re-processing In Progress/Done items
	jql := fmt.Sprintf("key = %s AND status not in (\"In Progress\", \"Done\", \"Resolved\", \"Project Signed Off\")", key)
	poller := orchestrator.NewJiraPoller(jClient, jql)

	dClient, err := docker.NewClient("recac-e2e")
	if err != nil {
		t.Fatalf("Failed to create docker client: %v", err)
	}

	// Session Manager
	tmpDir := t.TempDir()
	sm, err := runner.NewSessionManagerWithDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}

	// Spawner with explicit provider/model
	// Assuming OpenAI/GPT-3.5-turbo or similar for speed/cost if available. Or OpenRouter.
	provider := "openrouter"
	model := "mistralai/devstral-2512:free"
	spawner := orchestrator.NewDockerSpawner(logger, dClient, "recac-agent:e2e", "", poller, provider, model, sm)

	orch := orchestrator.New(poller, spawner, 5*time.Second)

	// 5. Run Orchestrator (Async)
	t.Log("Starting Orchestrator...")
	go func() {
		// Run with timeout (5 min max for agent to finish)
		fullCtx, cancelFull := context.WithTimeout(ctx, 5*time.Minute)
		defer cancelFull()
		if err := orch.Run(fullCtx, logger); err != nil && err != context.DeadlineExceeded {
			t.Logf("Orchestrator Run exited with error (expected cancellation): %v", err)
		}
	}()

	// Monitor Ticket Status to confirm pickup
	t.Log("Waiting for ticket pickup...")
	timeout := time.After(60 * time.Second)
	pickedUp := false
	for {
		select {
		case <-timeout:
			break
		default:
			ticket, _ := jClient.GetTicket(ctx, key)
			if ticket != nil {
				fields := ticket["fields"].(map[string]interface{})
				status := fields["status"].(map[string]interface{})
				if status["name"] == "In Progress" {
					t.Log("Ticket picked up (In Progress)!")
					pickedUp = true
					goto PickedUp
				}
			}
			time.Sleep(5 * time.Second)
		}
	}
PickedUp:

	if !pickedUp {
		t.Fatal("Orchestrator did not pick up ticket in time")
	}

	// Wait for Completion (Status = Done)
	t.Log("Waiting for code generation completion...")
	success := false
	waitStart := time.Now()
	for time.Since(waitStart) < 5*time.Minute {
		ticket, _ := jClient.GetTicket(ctx, key)
		if ticket != nil {
			fields := ticket["fields"].(map[string]interface{})
			status := fields["status"].(map[string]interface{})
			sName := status["name"].(string)
			// 'Done' or 'Project Signed Off'
			if sName == "Done" || sName == "Project Signed Off" || sName == "Resolved" {
				t.Logf("Ticket updated to %s!", sName)
				success = true
				break
			}
			if sName == "Failed" {
				t.Fatal("Ticket status is Failed. Agent execution failed.")
			}
		}
		time.Sleep(10 * time.Second)
	}

	if !success {
		// Dump logs if possible?
		// We can't easy get logs from deleted container unless we mount logs dir.
		t.Fatal("Agent did not complete ticket (status not Done)")
	}

	// 6. Verify Code
	t.Log("Verifying code in remote repo...")
	verifyDir := filepath.Join(os.TempDir(), fmt.Sprintf("verify_orch_%s", timestamp))

	gitToken := os.Getenv("GITHUB_API_KEY")
	if gitToken == "" {
		gitToken = os.Getenv("GITHUB_TOKEN")
	}
	// Sanitize token
	gitToken = strings.Trim(gitToken, "\"")

	authRepo := strings.Replace(repoURL, "https://github.com/", fmt.Sprintf("https://%s@github.com/", gitToken), 1)

	// Check if a branch containing the Key exists first
	lsRemote := exec.Command("git", "ls-remote", "--heads", authRepo)
	out, _ := lsRemote.Output()

	// Branch likely named `agent-epic/KEY` or `feature/KEY...` or `KEY`?
	// `recac start` creates branch `agent-epic/KEY` if epic, or `feature/KEY-summary`.
	// We'll search for just KEY.
	if !strings.Contains(string(out), key) {
		t.Logf("Warning: No branch found for ticket %s. Agent might have failed or merged?", key)
		// If merged to main, we check main.
	} else {
		t.Logf("Found branch for ticket %s", key)
	}

	// Since we expect it might be in a branch, let's clone that branch if found, or main.
	// For simplicity, finding the branch name from `out`
	branchName := "main"
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, key) {
			parts := strings.Fields(line)
			if len(parts) > 1 {
				ref := parts[1]
				branchName = strings.TrimPrefix(ref, "refs/heads/")
				t.Logf("Verifying branch: %s", branchName)
				break
			}
		}
	}

	cloneCmd := exec.Command("git", "clone", "--branch", branchName, authRepo, verifyDir)
	if out, err := cloneCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to clone for verification: %s", out)
	}

	// Check for file
	expectedFile := fmt.Sprintf("agent_%s.txt", timestamp)
	fullPath := filepath.Join(verifyDir, expectedFile)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		t.Errorf("Expected file %s not found in repo (branch %s)", expectedFile, branchName)
	} else {
		t.Log("SUCCESS: File found in repo!")
	}

	os.RemoveAll(verifyDir)
}
