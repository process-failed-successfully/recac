package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"recac/internal/db"

	"github.com/spf13/cobra"
)

var visualizeCmd = &cobra.Command{
	Use:   "visualize [feature_list_file]",
	Short: "Visualize the feature plan as a Mermaid graph",
	Long:  `Reads the feature list (default: feature_list.json) and generates a Mermaid flowchart.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputFile := "feature_list.json"
		if len(args) > 0 {
			inputFile = args[0]
		}

		content, err := os.ReadFile(inputFile)
		if err != nil {
			return fmt.Errorf("failed to read input file %s: %w", inputFile, err)
		}

		var featureList db.FeatureList
		if err := json.Unmarshal(content, &featureList); err != nil {
			return fmt.Errorf("failed to parse feature list: %w", err)
		}

		graph := generatePlanMermaid(featureList)
		fmt.Fprintln(cmd.OutOrStdout(), graph)
		return nil
	},
}

func init() {
	planCmd.AddCommand(visualizeCmd)
}

func generatePlanMermaid(fl db.FeatureList) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	for _, f := range fl.Features {
		safeID := SanitizeMermaidID(f.ID)

		// Truncate description for display
		// Use rune conversion for unicode safety
		runes := []rune(f.Description)
		desc := string(runes)
		if len(runes) > 30 {
			desc = string(runes[:27]) + "..."
		}
		// Escape quotes
		desc = strings.ReplaceAll(desc, "\"", "'")

		// Determine style
		style := ":::pending"
		status := strings.ToLower(f.Status)
		if status == "done" || status == "completed" {
			style = ":::done"
		} else if status == "in_progress" || status == "inprogress" {
			style = ":::inprogress"
		} else if status == "failed" {
			style = ":::failed"
		}

		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]%s\n", safeID, desc, style))

		for _, dep := range f.Dependencies.DependsOnIDs {
			safeDep := SanitizeMermaidID(dep)
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", safeDep, safeID))
		}
	}

	// Legend/Styles (matching graph.go)
	sb.WriteString("\n    classDef done fill:#90EE90,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef inprogress fill:#87CEEB,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef failed fill:#FF6347,stroke:#333,stroke-width:2px,color:black;\n")
	sb.WriteString("    classDef pending fill:#D3D3D3,stroke:#333,stroke-width:1px,color:black;\n")

	return sb.String()
}
