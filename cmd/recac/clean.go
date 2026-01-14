package main

import (
	"fmt"
	"os"
	"recac/internal/runner"

	"github.com/spf13/cobra"
)

var (
	cleanAll   bool
	cleanForce bool
)

// newCleanCmd creates the clean command
func newCleanCmd(sm ISessionManager) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean [session_name...]",
		Short: "Clean up session artifacts",
		Long: `Removes session files, logs, and the entire workspace directory for one or more sessions.
This command is for cleaning up the artifacts of completed or stopped sessions.
By default, it prompts for confirmation before deleting the workspace.`,
		Example: `  recac clean my-first-session
  recac clean session-1 session-2
  recac clean --all
  recac clean my-running-session --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClean(cmd, args, sm)
		},
	}
	cmd.Flags().BoolVar(&cleanAll, "all", false, "Clean all completed sessions")
	cmd.Flags().BoolVarP(&cleanForce, "force", "f", false, "Force removal without confirmation")
	return cmd
}


func runClean(cmd *cobra.Command, args []string, sm ISessionManager) error {
	if len(args) == 0 && !cleanAll {
		return fmt.Errorf("at least one session name is required, or use the --all flag")
	}
	if len(args) > 0 && cleanAll {
		return fmt.Errorf("cannot specify session names when using the --all flag")
	}

	cleaner := NewSessionCleaner(sm)

	var sessionsToClean []*runner.SessionState
	if cleanAll {
		allSessions, err := sm.ListSessions()
		if err != nil {
			return fmt.Errorf("failed to list sessions: %w", err)
		}
		for _, s := range allSessions {
			if s.Status == "completed" || s.Status == "stopped" || s.Status == "error" {
				sessionsToClean = append(sessionsToClean, s)
			}
		}
		if len(sessionsToClean) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No completed, stopped, or error sessions to clean.")
			return nil
		}
	} else {
		for _, name := range args {
			session, err := sm.LoadSession(name)
			if err != nil {
				return fmt.Errorf("failed to load session '%s': %w", name, err)
			}
			sessionsToClean = append(sessionsToClean, session)
		}
	}

	for _, session := range sessionsToClean {
		if err := cleaner.CleanSession(cmd, session, cleanForce); err != nil {
			// Return the first error to make testing simpler
			return fmt.Errorf("failed to clean session '%s': %w", session.Name, err)
		}
	}

	return nil
}

// SessionCleaner handles the logic for cleaning session artifacts.
type SessionCleaner struct {
	sm ISessionManager
}

func NewSessionCleaner(sm ISessionManager) *SessionCleaner {
	return &SessionCleaner{sm: sm}
}

func (sc *SessionCleaner) CleanSession(cmd *cobra.Command, session *runner.SessionState, force bool) error {
	// 1. Confirm workspace deletion
	if session.Workspace != "" {
		if _, err := os.Stat(session.Workspace); err == nil {
			confirmed, err := Confirm(
				cmd,
				fmt.Sprintf("This will permanently delete the workspace for session '%s' at:\n%s\nAre you sure?", session.Name, session.Workspace),
				force,
			)
			if err != nil {
				return fmt.Errorf("confirmation failed: %w", err)
			}
			if !confirmed {
				fmt.Fprintf(cmd.OutOrStdout(), "Skipping cleanup for session '%s'.\n", session.Name)
				return nil
			}
			// 2. Delete the workspace
			fmt.Fprintf(cmd.OutOrStdout(), "Deleting workspace: %s\n", session.Workspace)
			if err := os.RemoveAll(session.Workspace); err != nil {
				return fmt.Errorf("failed to remove workspace '%s': %w", session.Workspace, err)
			}
		}
	}

	// 3. Remove session files (.json, .log)
	fmt.Fprintf(cmd.OutOrStdout(), "Removing session files for '%s'\n", session.Name)
	if err := sc.sm.RemoveSession(session.Name, force); err != nil {
		if err == runner.ErrSessionRunning {
			return fmt.Errorf("session '%s' is running. Use --force to remove a running session and its artifacts", session.Name)
		}
		return fmt.Errorf("failed to remove session files: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Successfully cleaned session '%s'.\n", session.Name)
	return nil
}
