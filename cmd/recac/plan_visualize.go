package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"recac/internal/db"

	"github.com/spf13/cobra"
)

var planVisualizeCmd = &cobra.Command{
	Use:   "visualize [feature_list.json]",
	Short: "Visualize the feature plan as a dependency graph",
	Long:  `Generates a Mermaid flowchart showing features and their dependencies.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPlanVisualize,
}

func init() {
	planCmd.AddCommand(planVisualizeCmd)
	planVisualizeCmd.Flags().StringP("output", "o", "", "Output file (default: stdout)")
}

func runPlanVisualize(cmd *cobra.Command, args []string) error {
	inputFile := "feature_list.json"
	if len(args) > 0 {
		inputFile = args[0]
	}

	outputFile, _ := cmd.Flags().GetString("output")

	// Read Input
	content, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read feature list from %s: %w", inputFile, err)
	}

	var featureList db.FeatureList
	if err := json.Unmarshal(content, &featureList); err != nil {
		return fmt.Errorf("failed to parse feature list: %w", err)
	}

	// Generate Mermaid
	mermaid := generateMermaidPlan(featureList)

	// Output
	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(mermaid), 0644); err != nil {
			return fmt.Errorf("failed to write to %s: %w", outputFile, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Graph saved to %s\n", outputFile)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), mermaid)
	}

	return nil
}

func generateMermaidPlan(list db.FeatureList) string {
	var sb strings.Builder
	sb.WriteString("flowchart TD\n")
	sb.WriteString(fmt.Sprintf("    subgraph %s\n", sanitizePlanID(list.ProjectName)))
	sb.WriteString(fmt.Sprintf("    direction TB\n"))

	// 1. Nodes
	for _, f := range list.Features {
		// Clean description for label
		desc := f.Description
		if len(desc) > 30 {
			desc = desc[:27] + "..."
		}

		// Escape quotes in description
		desc = strings.ReplaceAll(desc, "\"", "'")

		// Node ID: just use f.ID if it's safe, otherwise sanitize
		nodeID := sanitizePlanID(f.ID)

		sb.WriteString(fmt.Sprintf("    %s[\"%s<br/>%s\"]\n", nodeID, f.ID, desc))

		// Styling based on status
		var style string
		switch strings.ToLower(f.Status) {
		case "completed", "done", "passed":
			style = "fill:#9f9,stroke:#333,stroke-width:2px"
		case "in_progress", "active":
			style = "fill:#9ff,stroke:#333,stroke-width:2px"
		case "failed", "error":
			style = "fill:#f99,stroke:#333,stroke-width:2px"
		default:
			// Pending or unknown
			style = "fill:#eee,stroke:#333,stroke-width:1px,stroke-dasharray: 5 5"
		}
		sb.WriteString(fmt.Sprintf("    style %s %s\n", nodeID, style))
	}

	// 2. Edges
	for _, f := range list.Features {
		nodeID := sanitizePlanID(f.ID)
		for _, dep := range f.Dependencies.DependsOnIDs {
			depID := sanitizePlanID(dep)
			// Check if depID exists in list? Ideally yes, but let's just draw edge.
			// Mermaid is forgiving if node isn't defined elsewhere, it creates it.
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", depID, nodeID))
		}
	}

	sb.WriteString("    end\n")
	return sb.String()
}

func sanitizePlanID(id string) string {
	// Mermaid IDs cannot contain spaces or special chars easily without quirks.
	// Replace invalid chars with underscore.
	invalid := []string{" ", "-", ".", "/", ":", "(", ")", "&", "[", "]", "*"}
	res := id
	for _, char := range invalid {
		res = strings.ReplaceAll(res, char, "_")
	}
	return res
}
