package main

import (
	"fmt"
	"recac/internal/runner"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(showCmd)
}

var showCmd = &cobra.Command{
	Use:     "show [SESSION_NAME]",
	Short:   "Show a summary of the work performed in a session",
	Long:    `Displays a git diff of the changes made during the specified session. This provides a clear and concise summary of the agent's work.`,
	Args:    cobra.ExactArgs(1),
	Aliases: []string{"view"},
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		session, err := sm.LoadSession(args[0])
		if err != nil {
			return fmt.Errorf("failed to load session %s: %w", args[0], err)
		}

		if session.StartCommitSHA == "" {
			return fmt.Errorf("session '%s' does not have a start commit SHA recorded", args[0])
		}

		endSHA, err := getShowCmdSessionEndSHA(session)
		if err != nil {
			return err
		}

		gitClient := gitNewClient()
		diff, err := gitClient.Diff(session.Workspace, session.StartCommitSHA, endSHA)
		if err != nil {
			return fmt.Errorf("failed to get git diff: %w", err)
		}

		cmd.Println(diff)
		return nil
	},
}

// getShowCmdSessionEndSHA determines the final commit SHA for a session.
// If the session is completed/stopped and has no explicit EndCommitSHA, it uses the current HEAD.
func getShowCmdSessionEndSHA(session *runner.SessionState) (string, error) {
	if session.EndCommitSHA != "" {
		return session.EndCommitSHA, nil
	}

	// If the session is complete but has no end SHA, use the current HEAD.
	if session.Status == "completed" || session.Status == "stopped" {
		gitClient := gitNewClient()
		currentSHA, err := gitClient.CurrentCommitSHA(session.Workspace)
		if err != nil {
			return "", fmt.Errorf("could not get current commit SHA for completed session '%s': %w", session.Name, err)
		}
		return currentSHA, nil
	}

	return "", fmt.Errorf("session '%s' is still running and does not have an end commit SHA", session.Name)
}
