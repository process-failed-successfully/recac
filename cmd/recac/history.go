package main

import (
	"fmt"
	"sort"
	"recac/internal/runner"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var askOne = survey.AskOne

// initHistoryCmd initializes the history command and adds it to the root command.
func initHistoryCmd(rootCmd *cobra.Command) {
	var fullLogs bool
	historyCmd := &cobra.Command{
		Use:   "history [session-name]",
		Short: "Show history of a specific session or a list of all sessions",
		Long: `Displays detailed history for a specific RECAC session.
If no session name is provided, it lists all sessions and prompts for a selection.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			sm, err := sessionManagerFactory()
			if err != nil {
				return fmt.Errorf("failed to initialize session manager: %w", err)
			}
			if len(args) > 0 {
				sessionName := args[0]
				session, err := sm.LoadSession(sessionName)
				if err != nil {
					return fmt.Errorf("failed to load session '%s': %w", sessionName, err)
				}
				return DisplaySessionDetail(cmd, session, fullLogs)
			}
			return runInteractiveHistory(cmd, sm, fullLogs)
		},
	}
	historyCmd.Flags().BoolVar(&fullLogs, "full-logs", false, "Display full log file content")
	rootCmd.AddCommand(historyCmd)
}

// runInteractiveHistory handles the interactive session selection.
func runInteractiveHistory(cmd *cobra.Command, sm ISessionManager, fullLogs bool) error {
	sessions, err := sm.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}
	if len(sessions) == 0 {
		cmd.Println("No sessions found.")
		return nil
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime.After(sessions[j].StartTime)
	})
	var displaySessions []string
	sessionMap := make(map[string]*runner.SessionState)
	for _, s := range sessions {
		status := s.Status
		if s.Status == "running" && !sm.IsProcessRunning(s.PID) {
			status = "completed"
		}
		display := fmt.Sprintf("%-30s (%-10s | %s)", s.Name, status, s.StartTime.Format("2006-01-02 15:04"))
		displaySessions = append(displaySessions, display)
		sessionMap[display] = s
	}
	var selectedDisplay string
	prompt := &survey.Select{
		Message:  "Select a session to view its history:",
		Options:  displaySessions,
		PageSize: 15,
	}
	if err := askOne(prompt, &selectedDisplay); err != nil {
		if err.Error() == "interrupt" {
			return nil // User cancelled
		}
		return fmt.Errorf("failed to select session: %w", err)
	}
	selectedSession := sessionMap[selectedDisplay]
	return DisplaySessionDetail(cmd, selectedSession, fullLogs)
}


