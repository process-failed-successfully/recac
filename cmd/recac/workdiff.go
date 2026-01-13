package main

import (
	"errors"
	"fmt"
	"recac/internal/runner"

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

func handleSingleSessionDiff(cmd *cobra.Command, sm ISessionManager, sessionName string) error {
	session, err := sm.LoadSession(sessionName)
	if err != nil {
		return fmt.Errorf("failed to load session %s: %w", sessionName, err)
	}

	if session.StartCommitSHA == "" {
		return fmt.Errorf("session '%s' does not have a start commit SHA recorded", sessionName)
	}

	endSHA, err := getSessionEndSHA(session)
	if err != nil {
		return err
	}

	gitClient := gitClientFactory()
	diff, err := gitClient.Diff(session.Workspace, session.StartCommitSHA, endSHA)
	if err != nil {
		return fmt.Errorf("failed to get git diff: %w", err)
	}

	cmd.Println(diff)
	return nil
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

// getSessionEndSHA determines the final commit SHA for a session.
// If the session is completed/stopped and has no explicit EndCommitSHA, it uses the current HEAD.
func getSessionEndSHA(session *runner.SessionState) (string, error) {
	if session.EndCommitSHA != "" {
		return session.EndCommitSHA, nil
	}

	// If the session is complete but has no end SHA, use the current HEAD.
	if session.Status == "completed" || session.Status == "stopped" {
		gitClient := gitClientFactory()
		currentSHA, err := gitClient.CurrentCommitSHA(session.Workspace)
		if err != nil {
			return "", fmt.Errorf("could not get current commit SHA for completed session '%s': %w", session.Name, err)
		}
		return currentSHA, nil
	}

	return "", fmt.Errorf("session '%s' is still running and does not have an end commit SHA", session.Name)
}
