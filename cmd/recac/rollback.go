package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback [session-name]",
	Short: "Rollback to a previous agent iteration",
	Long: `Reverts the repository and agent state to a previous iteration checkpoint.
It searches for git commits created by the agent ("chore: progress update") and allows you to reset the workspace to that point.
This is a destructive action for any changes made after the selected checkpoint.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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
			// Might be detached or error, warn but proceed?
			cmd.Printf("Warning: Could not determine current branch: %v\n", err)
		}

		// 4. Find Checkpoints
		// git log --grep="chore: progress update" --pretty=format:"%H|%s|%ar" -n 20
		logs, err := client.Log(workspace, "--grep=chore: progress update", "--pretty=format:%H|%s|%ar", "-n", "20")
		if err != nil {
			return fmt.Errorf("failed to search git log: %w", err)
		}

		if len(logs) == 0 {
			cmd.Println("No checkpoints found. The agent hasn't made any progress updates yet.")
			return nil
		}

		// 5. Prompt User
		options := make([]string, len(logs))
		commits := make(map[string]string) // Display -> SHA

		for i, line := range logs {
			parts := strings.SplitN(line, "|", 3)
			if len(parts) < 3 {
				continue
			}
			sha := parts[0]
			msg := parts[1]
			relTime := parts[2]

			display := fmt.Sprintf("%s - %s (%s)", sha[:7], msg, relTime)
			options[i] = display
			commits[display] = sha
		}

		var selected string
		prompt := &survey.Select{
			Message: "Select a checkpoint to rollback to:",
			Options: options,
		}

		if err := askOne(prompt, &selected); err != nil {
			return nil // User cancelled
		}

		targetSHA := commits[selected]
		cmd.Printf("Target Checkpoint: %s\n", targetSHA)

		// 6. Confirmation
		confirm := false
		confirmPrompt := &survey.Confirm{
			Message: "WARNING: This will hard reset the workspace to this commit and delete current agent memory. All subsequent changes will be LOST. Continue?",
			Default: false,
		}

		if err := askOne(confirmPrompt, &confirm); err != nil || !confirm {
			cmd.Println("Rollback cancelled.")
			return nil
		}

		// 7. Perform Rollback
		cmd.Println("Resetting git workspace...")
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

	// 3. List Sessions
	sm, err := sessionManagerFactory()
	if err != nil {
		return "", fmt.Errorf("failed to initialize session manager: %w", err)
	}

	sessions, err := sm.ListSessions()
	if err != nil || len(sessions) == 0 {
		return "", fmt.Errorf("current directory is not a git repo and no sessions found")
	}

	// Prompt user
	var options []string
	sessionMap := make(map[string]string)
	for _, s := range sessions {
		display := fmt.Sprintf("%s (%s)", s.Name, s.Status)
		options = append(options, display)
		sessionMap[display] = s.Workspace
	}

	var selected string
	prompt := &survey.Select{
		Message: "Select a session to rollback:",
		Options: options,
	}
	if err := askOne(prompt, &selected); err != nil {
		return "", fmt.Errorf("selection cancelled")
	}

	return sessionMap[selected], nil
}
