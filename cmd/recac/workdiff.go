package main

import (
	"fmt"
	"recac/internal/git"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(workdiffCmd)
}

var workdiffCmd = &cobra.Command{
	Use:   "workdiff [session-name]",
	Short: "Show a git diff of the work done in a session",
	Long: `Displays the git diff between the starting and ending commits of a completed session.
This command helps you review the exact changes made by the agent during its run.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to initialize session manager: %w", err)
		}

		session, err := sm.LoadSession(sessionName)
		if err != nil {
			return fmt.Errorf("failed to load session %s: %w", sessionName, err)
		}

		if session.StartCommitSHA == "" {
			return fmt.Errorf("session '%s' does not have a start commit SHA recorded", sessionName)
		}

		endSHA := session.EndCommitSHA
		if endSHA == "" {
			// If the session is complete but has no end SHA, use the current HEAD.
			if session.Status == "completed" || session.Status == "stopped" {
				gitClient := git.NewClient()
				currentSHA, err := gitClient.CurrentCommitSHA(session.Workspace)
				if err != nil {
					return fmt.Errorf("could not get current commit SHA for completed session: %w", err)
				}
				endSHA = currentSHA
			} else {
				return fmt.Errorf("session '%s' is still running and does not have an end commit SHA", sessionName)
			}
		}

		gitClient := git.NewClient()
		diff, err := gitClient.Diff(session.Workspace, session.StartCommitSHA, endSHA)
		if err != nil {
			return fmt.Errorf("failed to get git diff: %w", err)
		}

		cmd.Println(diff)
		return nil
	},
}
