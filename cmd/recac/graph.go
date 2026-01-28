package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/runner"
	"sort"

	"github.com/spf13/cobra"
)

var graphCmd = &cobra.Command{
	Use:   "graph [SESSION_NAME]",
	Short: "Visualize the task dependency graph",
	Long:  `Generates a Mermaid flowchart of the task dependency graph for a session.`,
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
			sessions, err := sm.ListSessions()
			if err != nil {
				return fmt.Errorf("failed to list sessions: %w", err)
			}
			if len(sessions) == 0 {
				return fmt.Errorf("no sessions found")
			}
			sort.Slice(sessions, func(i, j int) bool {
				return sessions[i].StartTime.After(sessions[j].StartTime)
			})
			session = sessions[0]
		}

		// Initialize DB Store
		dbPath := filepath.Join(session.Workspace, ".recac.db")
		store, err := db.NewStore(db.StoreConfig{
			Type:             "sqlite",
			ConnectionString: dbPath,
		})
		if err != nil {
			return fmt.Errorf("failed to open database at %s: %w", dbPath, err)
		}

		// Load Features
		// We assume the project name matches the session name.
		// If not, we might need a fallback or flag, but this covers 99% of cases.
		projectName := session.Name

		// If the session name doesn't work, try "default" or the directory name
		content, err := store.GetFeatures(projectName)
		if err != nil || content == "" {
			// Try directory name
			projectName = filepath.Base(session.Workspace)
			content, err = store.GetFeatures(projectName)
		}

		if err != nil {
			return fmt.Errorf("failed to load features from DB: %w", err)
		}
		if content == "" {
			return fmt.Errorf("no features found for project '%s'", projectName)
		}

		var fl db.FeatureList
		if err := json.Unmarshal([]byte(content), &fl); err != nil {
			return fmt.Errorf("failed to parse features: %w", err)
		}

		// Build Graph
		g := runner.NewTaskGraph()
		if err := g.LoadFromFeatures(fl.Features); err != nil {
			return fmt.Errorf("failed to build graph: %w", err)
		}

		// Generate Mermaid
		fmt.Fprintln(cmd.OutOrStdout(), GenerateMermaidGraph(g))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(graphCmd)
}
