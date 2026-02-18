package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	rollbackCommit string
	rollbackForce  bool
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback [session-name]",
	Short: "Rollback to a previous agent iteration",
	Long: `Reverts the repository and agent state to a previous iteration checkpoint.
It searches for git commits created by the agent ("chore: progress update") and resets the workspace to the specified commit.
This is a destructive action for any changes made after the selected checkpoint.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if rollbackCommit == "" {
			return fmt.Errorf("--commit is required for rollback")
		}

		// 1. Determine Workspace
		workspace, err := resolveWorkspace(cmd, args)
		if err != nil {
			return err
		}

		cmd.Printf("Analyzing workspace: %s\n", workspace)

		// 2. Initialize Git Client
		client := gitClientFactory()
		if !client.RepoExists(workspace) {
			return fmt.Errorf("workspace is not a git repository")
		}

		// 3. Get Current Branch (to restore pointer later)
		currentBranch, err := client.CurrentBranch(workspace)
		if err != nil {
			cmd.Printf("Warning: Could not determine current branch: %v\n", err)
		}

		targetSHA := rollbackCommit

		// 6. Confirmation check
		if !rollbackForce {
			return fmt.Errorf("this is a destructive action. Use --force to confirm rollback to %s", targetSHA)
		}

		cmd.Printf("Target Checkpoint: %s\n", targetSHA)
		// Logic: Checkout SHA (detached), then CheckoutNewBranch (reset branch pointer)
		if err := client.Checkout(workspace, targetSHA); err != nil {
			return fmt.Errorf("failed to checkout checkpoint %s: %w", targetSHA, err)
		}

		if currentBranch != "" && currentBranch != "HEAD" {
			if err := client.CheckoutNewBranch(workspace, currentBranch); err != nil {
				return fmt.Errorf("failed to reset branch %s to checkpoint: %w", currentBranch, err)
			}
			cmd.Printf("Branch '%s' reset to %s\n", currentBranch, targetSHA[:7])
		} else {
			cmd.Println("Workspace reset to checkpoint (Detached HEAD)")
		}

		// 8. Delete Agent State
		cmd.Println("Clearing agent state...")
		stateFiles := []string{".agent_state.json", ".recac.db"} // Maybe db too?

		for _, f := range stateFiles {
			path := filepath.Join(workspace, f)
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				cmd.Printf("Warning: Failed to delete %s: %v\n", f, err)
			} else if err == nil {
				cmd.Printf("Deleted %s\n", f)
			}
		}

		cmd.Println("Rollback complete. You can now resume the session.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(rollbackCmd)
	rollbackCmd.Flags().StringVar(&rollbackCommit, "commit", "", "Target commit SHA to rollback to")
	rollbackCmd.Flags().BoolVar(&rollbackForce, "force", false, "Force rollback (bypass confirmation)")
}

func resolveWorkspace(cmd *cobra.Command, args []string) (string, error) {
	// 1. If argument provided, load session
	if len(args) > 0 {
		sessionName := args[0]
		sm, err := sessionManagerFactory()
		if err != nil {
			return "", fmt.Errorf("failed to initialize session manager: %w", err)
		}
		session, err := sm.LoadSession(sessionName)
		if err != nil {
			return "", fmt.Errorf("failed to load session '%s': %w", sessionName, err)
		}
		return session.Workspace, nil
	}

	// 2. Try current directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Check if CWD is a git repo
	client := gitClientFactory()
	if client.RepoExists(cwd) {
		return cwd, nil
	}

	return "", fmt.Errorf("current directory is not a git repo and no session name provided")
}
