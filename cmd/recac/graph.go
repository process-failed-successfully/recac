package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/runner"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newGraphCmd() *cobra.Command {
	return &cobra.Command{
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
			defer store.Close()

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
			fmt.Fprintln(cmd.OutOrStdout(), generateMermaid(g))
			return nil
		},
	}
}

func init() {
	rootCmd.AddCommand(newGraphCmd())
}

func generateMermaid(g *runner.TaskGraph) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Collect nodes to ensure deterministic output
	var nodes []*runner.TaskNode
	for _, node := range g.Nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	for _, node := range nodes {
		// Style based on status
		style := ""
		switch node.Status {
		case runner.TaskDone:
			style = ":::done"
		case runner.TaskInProgress:
			style = ":::inprogress"
		case runner.TaskFailed:
			style = ":::failed"
		case runner.TaskReady:
			style = ":::ready"
		default: // Pending
			style = ":::pending"
		}

		// Sanitize ID and Name for Mermaid
		safeID := sanitizeMermaidID(node.ID)
		safeName := strings.ReplaceAll(node.Name, "\"", "'")
		safeName = strings.ReplaceAll(safeName, "\n", " ")
		if len(safeName) > 30 {
			safeName = safeName[:27] + "..."
		}

		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]%s\n", safeID, safeName, style))

		for _, depID := range node.Dependencies {
			safeDepID := sanitizeMermaidID(depID)
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", safeDepID, safeID))
		}
	}

	// Legend/Styles
	sb.WriteString("\n    classDef done fill:#90EE90,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef inprogress fill:#87CEEB,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef failed fill:#FF6347,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef ready fill:#FFD700,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef pending fill:#D3D3D3,stroke:#333,stroke-width:1px,color:black;\n")

	return sb.String()
}

func sanitizeMermaidID(id string) string {
	id = strings.ReplaceAll(id, "-", "_")
	id = strings.ReplaceAll(id, " ", "_")
	id = strings.ReplaceAll(id, ".", "_")
	return id
}
