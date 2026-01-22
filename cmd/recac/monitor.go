package main

import (
	"fmt"
	"recac/internal/model"
	"recac/internal/ui"

	"github.com/spf13/cobra"
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Interactive session control center",
	Long:  `Launches a Terminal UI (TUI) to monitor and control active sessions. Allows listing, killing, pausing, resuming, and viewing logs of sessions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		// Helper to adapt unified sessions
		getSessions := func() ([]model.UnifiedSession, error) {
			// Reuse the logic from ps.go's getUnifiedSessions if possible, or reimplement basic local fetch
			// To keep it clean and avoid circular dependency on ps.go functions (which are package private to main),
			// we will use a basic implementation here or call the shared one if we refactor.
			// Since refactoring is risky, I will implement a safe version here that calls the same helpers as ps.go

			// We need a dummy command and filters for getUnifiedSessions
			// But getUnifiedSessions is in ps.go (package main), so we can call it!
			// We just need to ensure filters are correct.
			filters := model.PsFilters{
				Remote:   false, // Default to local only for safety, or maybe true? Let's say false for now unless flagged.
				LogLines: 0,
			}
			return getUnifiedSessions(cmd, filters)
		}

		callbacks := ui.ActionCallbacks{
			GetSessions: getSessions,
			Stop: func(name string) error {
				return sm.StopSession(name)
			},
			Pause: func(name string) error {
				return sm.PauseSession(name)
			},
			Resume: func(name string) error {
				return sm.ResumeSession(name)
			},
			GetLogs: func(name string) (string, error) {
				return sm.GetSessionLogContent(name, 1000)
			},
		}

		return ui.StartMonitorDashboard(callbacks)
	},
}

func init() {
	rootCmd.AddCommand(monitorCmd)
}
