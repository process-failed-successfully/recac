package main

import (
	"fmt"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/runner"
	"recac/internal/web"
	"sort"

	"github.com/spf13/cobra"
)

var (
	webPort int
)

var webCmd = &cobra.Command{
	Use:   "web [SESSION_NAME]",
	Short: "Start a web dashboard to visualize project status",
	Long:  `Starts a local web server that provides a visual dashboard for the project, including task graphs, feature status, and more.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		var session *runner.SessionState
		if len(args) == 1 {
			session, err = sm.LoadSession(args[0])
			if err != nil {
				return err
			}
		} else {
			// Try to find the most recent session
			sessions, err := sm.ListSessions()
			if err != nil {
				// If no sessions, maybe we are in a repo root without a session?
				// Just try to open .recac.db in current dir
				session = &runner.SessionState{
					Workspace: ".",
					Name:      "default",
				}
			} else if len(sessions) > 0 {
				sort.Slice(sessions, func(i, j int) bool {
					return sessions[i].StartTime.After(sessions[j].StartTime)
				})
				session = sessions[0]
			} else {
				session = &runner.SessionState{
					Workspace: ".",
					Name:      "default",
				}
			}
		}

		dbPath := filepath.Join(session.Workspace, ".recac.db")
		fmt.Printf("Using database: %s\n", dbPath)

		store, err := db.NewStore(db.StoreConfig{
			Type:             "sqlite",
			ConnectionString: dbPath,
		})
		if err != nil {
			return fmt.Errorf("failed to open database at %s: %w", dbPath, err)
		}
		defer store.Close()

		// Get project name from session or default
		projectID := session.Name
		if projectID == "" {
			projectID = "default"
		}

		server := web.NewServer(store, webPort, projectID)
		return server.Start()
	},
}

func init() {
	rootCmd.AddCommand(webCmd)
	webCmd.Flags().IntVarP(&webPort, "port", "p", 8080, "Port to run the web server on")
}
