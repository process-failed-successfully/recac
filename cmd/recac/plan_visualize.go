package main

import (
	"encoding/json"
	"fmt"
	"os"
	"recac/internal/db"
	"strings"

	"github.com/spf13/cobra"
)

var planVisualizeCmd = &cobra.Command{
	Use:   "visualize [feature_list.json]",
	Short: "Visualize the plan as a Mermaid graph",
	Long:  `Generates a Mermaid flowchart of the features and their dependencies.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPlanVisualize,
}

func init() {
	// Attach to the shared planCmd from plan.go
	planCmd.AddCommand(planVisualizeCmd)
}

func runPlanVisualize(cmd *cobra.Command, args []string) error {
	inputFile := "feature_list.json"
	if len(args) > 0 {
		inputFile = args[0]
	}

	content, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read feature list %s: %w", inputFile, err)
	}

	var featureList db.FeatureList
	if err := json.Unmarshal(content, &featureList); err != nil {
		return fmt.Errorf("failed to parse feature list: %w", err)
	}

	mermaid := generateMermaidPlan(featureList)
	fmt.Fprintln(cmd.OutOrStdout(), mermaid)
	return nil
}

func generateMermaidPlan(list db.FeatureList) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Helper to sanitize ID
	sanitize := func(s string) string {
		return strings.ReplaceAll(s, "-", "_")
	}

	// Helper to truncate text
	truncate := func(s string, n int) string {
		runes := []rune(s)
		if len(runes) > n {
			return string(runes[:n-3]) + "..."
		}
		return s
	}

	for _, f := range list.Features {
		safeID := sanitize(f.ID)
		desc := truncate(f.Description, 30)

		// Node definition with label
		sb.WriteString(fmt.Sprintf("    %s[\"%s: %s\"]\n", safeID, f.ID, desc))

		// Styling based on status
		var style string
		switch f.Status {
		case "completed", "done":
			style = "fill:#9f9,stroke:#333,stroke-width:2px" // Green
		case "in-progress", "started":
			style = "fill:#99f,stroke:#333,stroke-width:2px" // Blue
		case "failed":
			style = "fill:#f99,stroke:#333,stroke-width:2px" // Red
		default: // pending, etc.
			style = "fill:#fff,stroke:#333,stroke-width:1px,stroke-dasharray: 5 5"
		}
		sb.WriteString(fmt.Sprintf("    style %s %s\n", safeID, style))

		// Dependencies
		for _, depID := range f.Dependencies.DependsOnIDs {
			safeDepID := sanitize(depID)
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", safeDepID, safeID))
		}
	}

	return sb.String()
}
