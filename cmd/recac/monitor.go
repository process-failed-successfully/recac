package main

import (
	"context"
	"fmt"
	"recac/internal/model"
	"recac/internal/ui"

	"github.com/spf13/cobra"
)

// startMonitorDashboardFunc is a variable to allow mocking in tests
var startMonitorDashboardFunc = ui.StartMonitorDashboard

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
				Remote:   true,
				LogLines: 0,
			}
			return getUnifiedSessions(cmd, filters)
		}

		callbacks := ui.ActionCallbacks{
			GetSessions: getSessions,
			Stop: func(session model.UnifiedSession) error {
				if session.Location == "k8s" {
					client, err := k8sClientFactory()
					if err != nil {
						return err
					}
					return client.DeletePod(context.Background(), session.ID)
				}
				return sm.StopSession(session.Name)
			},
			Pause: func(session model.UnifiedSession) error {
				if session.Location == "k8s" {
					return fmt.Errorf("pausing k8s sessions is not supported")
				}
				return sm.PauseSession(session.Name)
			},
			Resume: func(session model.UnifiedSession) error {
				if session.Location == "k8s" {
					return fmt.Errorf("resuming k8s sessions is not supported")
				}
				return sm.ResumeSession(session.Name)
			},
			GetLogs: func(session model.UnifiedSession) (string, error) {
				if session.Location == "k8s" {
					client, err := k8sClientFactory()
					if err != nil {
						return "", err
					}
					return client.GetPodLogs(context.Background(), session.ID, 1000)
				}
				return sm.GetSessionLogContent(session.Name, 1000)
			},
		}

		return startMonitorDashboardFunc(callbacks)
	},
}

func init() {
	rootCmd.AddCommand(monitorCmd)
}
