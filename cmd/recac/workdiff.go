package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(workdiffCmd)
}

var workdiffCmd = &cobra.Command{
	Use:     "workdiff [session-name] | [session-a] [session-b]",
	Short:   "Show a git diff of work; alias for single-session 'show'",
	Aliases: []string{"show"},
	Long: `Displays the git diff between the starting and ending commits of a completed session.
This is the primary command for reviewing work. The 'show' alias is provided for convenience.

If a single session name is provided, it shows the diff for that session.
If two session names are provided, it displays the git diff between the final states of those two sessions.`,
	Args: cobra.ArbitraryArgs, // Let RunE handle more nuanced validation
	RunE: func(cmd *cobra.Command, args []string) error {
		numArgs := len(args)
		calledAs := cmd.CalledAs()

		// Custom argument validation
		if calledAs == "show" {
			if numArgs != 1 {
				return errors.New("the 'show' alias requires exactly one session name")
			}
		} else { // "workdiff"
			if numArgs < 1 || numArgs > 2 {
				return fmt.Errorf("accepts between 1 and 2 arg(s), received %d", numArgs)
			}
		}

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to initialize session manager: %w", err)
		}

		if numArgs == 1 {
			return handleSingleSessionDiff(cmd, sm, args[0])
		}
		return handleTwoSessionDiff(cmd, sm, args[0], args[1])
	},
}

func handleTwoSessionDiff(cmd *cobra.Command, sm ISessionManager, sessionAName, sessionBName string) error {
	sessionA, err := sm.LoadSession(sessionAName)
	if err != nil {
		return fmt.Errorf("failed to load session %s: %w", sessionAName, err)
	}

	sessionB, err := sm.LoadSession(sessionBName)
	if err != nil {
		return fmt.Errorf("failed to load session %s: %w", sessionBName, err)
	}

	endSHA_A, err := getSessionEndSHA(sessionA)
	if err != nil {
		return err
	}

	endSHA_B, err := getSessionEndSHA(sessionB)
	if err != nil {
		return err
	}

	// Assuming both sessions operate on the same workspace.
	// If not, this logic might need to be adjusted.
	workspace := sessionA.Workspace
	if workspace == "" {
		workspace = sessionB.Workspace // Fallback
	}
	if workspace == "" {
		return fmt.Errorf("cannot determine workspace for diff")
	}

	gitClient := gitClientFactory()
	diff, err := gitClient.Diff(workspace, endSHA_A, endSHA_B)
	if err != nil {
		return fmt.Errorf("failed to get git diff between sessions: %w", err)
	}

	cmd.Println(diff)
	return nil
}

