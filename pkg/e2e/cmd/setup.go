package cmd

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"recac/internal/jira"
	"recac/pkg/e2e/manager"
	"recac/pkg/e2e/scenarios"
	"recac/pkg/e2e/state"

	"github.com/joho/godotenv"
)

var (
	defaultRepo = "192.168.0.55:5000/recac-e2e"
	repoURL     = "https://github.com/process-failed-successfully/recac-jira-e2e"
)

func RunSetup(args []string) error {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	var (
		scenarioName string
		provider     string
		model        string
		targetRepo   string
		stateFile    string
	)

	fs.StringVar(&scenarioName, "scenario", "prime-python", "Scenario to run")
	fs.StringVar(&provider, "provider", "openrouter", "AI Provider")
	fs.StringVar(&model, "model", "mistralai/devstral-2512:free", "AI Model")
	fs.StringVar(&targetRepo, "repo-url", repoURL, "Target Git repository for the agent")
	fs.StringVar(&stateFile, "state-file", "e2e_state.json", "Path to save state file")
	fs.Parse(args)

	_ = godotenv.Load()

	if targetRepo != "" {
		repoURL = targetRepo
	}

	// Validate Env (Shared logic, could be extracted)
	required := []string{"JIRA_URL", "JIRA_USERNAME", "JIRA_API_TOKEN"}
	for _, env := range required {
		if os.Getenv(env) == "" {
			return fmt.Errorf("missing required env var: %s", env)
		}
	}

	// Fallback/Default for API key if token not set
	if os.Getenv("JIRA_API_TOKEN") == "" && os.Getenv("JIRA_API_KEY") != "" {
		os.Setenv("JIRA_API_TOKEN", os.Getenv("JIRA_API_KEY"))
	}
	projectKey := os.Getenv("JIRA_PROJECT_KEY")

	ctx := context.Background()

	// Fallback for missing JIRA_PROJECT_KEY
	if projectKey == "" {
		log.Println("JIRA_PROJECT_KEY not set. Attempting to fetch default project...")
		tmpClient := jira.NewClient(os.Getenv("JIRA_URL"), os.Getenv("JIRA_USERNAME"), os.Getenv("JIRA_API_TOKEN"))
		var err error
		projectKey, err = tmpClient.GetFirstProjectKey(ctx)
		if err != nil {
			return fmt.Errorf("missing JIRA_PROJECT_KEY and failed to fetch default: %w", err)
		}
		log.Printf("Using default project key: %s", projectKey)
	}

	mgr := manager.NewJiraManager(os.Getenv("JIRA_URL"), os.Getenv("JIRA_USERNAME"), os.Getenv("JIRA_API_TOKEN"), projectKey)

	// 1. Setup Jira
	log.Println("=== Setting up Jira Scenario ===")
	if _, ok := scenarios.Registry[scenarioName]; !ok {
		return fmt.Errorf("unknown scenario: %s", scenarioName)
	}

	label, ticketMap, err := mgr.GenerateScenario(ctx, scenarioName, repoURL, provider, model)
	if err != nil {
		return fmt.Errorf("failed to generate scenario: %w", err)
	}
	log.Printf("Scenario generated with label: %s", label)

	// 2. Prepare Repository
	log.Println("=== Preparing Repository (Cleaning stale branches) ===")
	if err := prepareRepo(repoURL, ticketMap); err != nil {
		log.Printf("Warning: Failed to prepare repository: %v", err)
	}

	// 3. Save State
	e2eCtx := &state.E2EContext{
		ID:             label, // Using label as ID for now
		ScenarioName:   scenarioName,
		JiraProjectKey: projectKey,
		JiraLabel:      label,
		TicketMap:      ticketMap,
		RepoURL:        repoURL,
		Provider:       provider,
		Model:          model,
	}

	if err := e2eCtx.Save(stateFile); err != nil {
		return fmt.Errorf("failed to save state file: %w", err)
	}

	log.Printf("Setup complete. State saved to %s", stateFile)
	return nil
}

// prepareRepo is duplicated for now, should be moved to pkg/e2e/git or similar
func prepareRepo(repoURL string, ticketMap map[string]string) error {
	token := os.Getenv("GITHUB_API_KEY")
	repoURL = strings.TrimSuffix(repoURL, "/")
	authRepo := repoURL
	if token != "" && !strings.Contains(repoURL, "@") {
		authRepo = strings.Replace(repoURL, "https://", fmt.Sprintf("https://x-access-token:%s@", token), 1)
	}

	// 1. Get all remote branches using ls-remote (fast, no clone needed)
	log.Printf("Checking remote branches for %s...", repoURL)
	cmd := exec.Command("git", "ls-remote", "--heads", authRepo)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to ls-remote: %w\nOutput: %s", err, string(output))
	}

	lines := strings.Split(string(output), "\n")
	var branchesToDelete []string
	for _, line := range lines {
		// line format: <sha>\trefs/heads/<branch>
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		ref := parts[1]
		if strings.HasPrefix(ref, "refs/heads/agent/") {
			branch := strings.TrimPrefix(ref, "refs/heads/")
			branchesToDelete = append(branchesToDelete, branch)
		}
	}

	if len(branchesToDelete) == 0 {
		return nil
	}

	// 2. Delete the branches
	log.Printf("Found %d stale agent branches to delete", len(branchesToDelete))
	for _, branch := range branchesToDelete {
		log.Printf("Deleting remote branch: %s", branch)
		delCmd := exec.Command("git", "push", authRepo, "--delete", branch)
		if out, err := delCmd.CombinedOutput(); err != nil {
			log.Printf("Warning: Failed to delete branch %s: %v\nOutput: %s", branch, err, string(out))
		}
	}

	return nil
}
