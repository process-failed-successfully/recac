package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"recac/internal/db"

	"github.com/spf13/cobra"
)

var planVisualizeCmd = &cobra.Command{
	Use:   "visualize [feature_list_file]",
	Short: "Visualize the feature plan as a Mermaid flowchart",
	Long:  `Generates a Mermaid flowchart from the feature list JSON file (default: feature_list.json).`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPlanVisualize,
}

func init() {
	planCmd.AddCommand(planVisualizeCmd)
}

func runPlanVisualize(cmd *cobra.Command, args []string) error {
	file := "feature_list.json"
	if len(args) > 0 {
		file = args[0]
	}

	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read feature list file %s: %w", file, err)
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
	sb.WriteString(fmt.Sprintf("    title %s Plan\n", list.ProjectName))

	// Sort features by ID for determinism
	sort.Slice(list.Features, func(i, j int) bool {
		return list.Features[i].ID < list.Features[j].ID
	})

	for _, f := range list.Features {
		safeID := sanitizeMermaidID(f.ID)

		// Truncate description for the label
		desc := f.Description
		if len(desc) > 30 {
			desc = string([]rune(desc)[:27]) + "..."
		}
		// Escape quotes
		desc = strings.ReplaceAll(desc, "\"", "'")

		label := fmt.Sprintf("%s<br/>(%s)", desc, f.Priority)
		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", safeID, label))

		// Style based on Priority
		style := ""
		switch strings.ToLower(f.Priority) {
		case "poc", "high":
			style = "fill:#ffcccc,stroke:#cc0000" // Red-ish
		case "mvp", "medium":
			style = "fill:#ffffcc,stroke:#cccc00" // Yellow-ish
		case "production", "low":
			style = "fill:#ccffcc,stroke:#00cc00" // Green-ish
		default:
			style = "fill:#f9f9f9,stroke:#333"
		}

		// Also consider status
		if strings.ToLower(f.Status) == "completed" || strings.ToLower(f.Status) == "done" {
			style = "fill:#ccccff,stroke:#0000cc" // Blue-ish for done
		}

		sb.WriteString(fmt.Sprintf("    style %s %s\n", safeID, style))

		// Dependencies
		for _, depID := range f.Dependencies.DependsOnIDs {
			safeDepID := sanitizeMermaidID(depID)
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", safeDepID, safeID))
		}
	}

	return sb.String()
}
