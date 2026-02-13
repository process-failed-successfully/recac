package main

import (
	"errors"
	"fmt"
	"os"
	"recac/internal/agent"
	"recac/internal/runner"
	"recac/internal/ui"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(attachCmd)
}

var attachCmd = &cobra.Command{
	Use:   "attach [session-name]",
	Short: "Re-attach to a running session",
	Long:  `Re-attach to a running session to view its output in real-time.
This opens a TUI dashboard showing the session status and streaming logs.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		session, err := sm.LoadSession(sessionName)
		if err != nil {
			return fmt.Errorf("session not found: %w", err)
		}

		if session.Status != "running" {
			// Optional: Allow attaching to completed sessions just to see logs?
			// The user can use 'logs' for that. But 'attach' implies active.
			// Let's warn but proceed if they really want? No, let's stick to running for now,
			// or just show it anyway. The TUI handles completed sessions fine (status will be 'completed').
			// But 'attach' usually means attaching to a process.
			// However, for viewing the dashboard, it's fine.
			// Let's print a message but continue.
			fmt.Printf("Note: Session '%s' is %s (not running).\n", sessionName, session.Status)
		}

		// Configure the status fetcher for the UI
		ui.GetSessionStatus = func(name string) (*runner.SessionState, *agent.State, string, error) {
			// We use the captured 'sm'
			sess, err := sm.LoadSession(name)
			if err != nil {
				return nil, nil, "", err
			}

			diffStat, err := sm.GetSessionGitDiffStat(name)
			if err != nil {
				diffStat = ""
			}

			st, err := loadAgentState(sess.AgentStateFile)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return sess, nil, diffStat, nil
				}
				return sess, nil, diffStat, err
			}
			return sess, st, diffStat, nil
		}

		logFile, err := sm.GetSessionLogs(sessionName)
		if err != nil {
			return fmt.Errorf("failed to get log path: %w", err)
		}

		// Verify log file exists
		if _, err := os.Stat(logFile); os.IsNotExist(err) {
			return fmt.Errorf("log file not found at %s", logFile)
		}

		fmt.Printf("Attaching to session '%s'...\n", sessionName)

		// Start the TUI
		if err := ui.StartAttachDashboard(sessionName, logFile); err != nil {
			return fmt.Errorf("dashboard error: %w", err)
		}

		return nil
	},
}
