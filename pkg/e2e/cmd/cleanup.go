package cmd

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"recac/pkg/e2e/manager"
	"recac/pkg/e2e/state"

	"github.com/joho/godotenv"
)

func RunCleanup(args []string) error {
	fs := flag.NewFlagSet("cleanup", flag.ExitOnError)
	var stateFile string
	fs.StringVar(&stateFile, "state-file", "e2e_state.json", "Path to state file")
	fs.Parse(args)

	_ = godotenv.Load()

	e2eCtx, err := state.Load(stateFile)
	if err != nil {
		return fmt.Errorf("failed to load state file: %w", err)
	}

	mgr := manager.NewJiraManager(
		os.Getenv("JIRA_URL"),
		os.Getenv("JIRA_USERNAME"),
		os.Getenv("JIRA_API_TOKEN"),
		e2eCtx.JiraProjectKey,
	)

	log.Println("=== Cleaning up Jira ===")
	// Note: mgr.Cleanup uses the label to find and archive tickets
	if err := mgr.Cleanup(context.Background(), e2eCtx.JiraLabel); err != nil {
		return fmt.Errorf("failed cleanup: %w", err)
	}

	// Helper to also remove the Helm release if we added that to context, 
	// but for now let's stick to Jira cleanup as the primary goal here.
	// Helm cleanup is often handled by 'helm uninstall' which is easy to run manually,
	// but we could add it here if we stored release name.
	
	if e2eCtx.ReleaseName != "" && e2eCtx.Namespace != "" {
		log.Printf("Uninstalling Helm release: %s", e2eCtx.ReleaseName)
		if err := runCommand("helm", "uninstall", e2eCtx.ReleaseName, "--namespace", e2eCtx.Namespace); err != nil {
			log.Printf("Warning: Failed to uninstall helm release: %v", err)
		}
	}

	return nil
}

// Ensure runCommand is available if it wasn't exported from verify.go.
// Since these are in the same package 'cmd', they share unexported identifiers.
// verify.go defined runCommand, so we are good.
